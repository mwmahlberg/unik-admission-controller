package main

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const AnnotationNcpSnatPool = "ncp/snat_pool"

type ValidationHandlerV1 interface {
	validate(admissionv1.AdmissionReview) *admissionv1.AdmissionResponse
}

// AdmitHandler is a wrapper around an admission handler function.
// Using it allows us to implement various versions of the admission API.
type admitHandlerV1 struct {
	clientset *kubernetes.Clientset
	logger    *zap.Logger
}

var serviceRessource = metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "services"}

func ValidationHandler(logger *zap.Logger) *admitHandlerV1 {

	h := &admitHandlerV1{
		logger: logger,
	}

	config, err := rest.InClusterConfig()

	if err != nil {
		logger.Panic("Could not create kubernetes client", zap.String("component", "in-cluster-config"), zap.Error(err))
		return nil
	}

	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		logger.Panic("Could not create kubernetes client", zap.String("component", "clientset"), zap.Error(err))
	}
	h.clientset = clientset
	return h
}
func (h *admitHandlerV1) validate(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	h.logger.Debug("Validating request")

	if ar.Request.Resource != serviceRessource {
		return &admissionv1.AdmissionResponse{
			UID:      ar.Request.UID,
			Allowed:  true,
			Warnings: []string{"this is not a service"},
		}
	}
	h.logger.Debug("Request is a service", zap.String("group", ar.Request.Kind.Group), zap.String("version", ar.Request.Kind.Version), zap.String("kind", ar.Request.Kind.Kind))

	svc := corev1.Service{}
	_, _, _ = deserializer.Decode(ar.Request.Object.Raw, nil, &svc)

	toSearch, present := svc.Annotations[AnnotationNcpSnatPool]
	if !present {
		h.logger.Debug("Annotation not present in service", zap.String("annotation", AnnotationNcpSnatPool))
		return &admissionv1.AdmissionResponse{
			UID:     ar.Request.UID,
			Allowed: true,
		}
	}

	h.logger.Debug("Found annotation, checking existing services", zap.String("annotation", AnnotationNcpSnatPool), zap.String("value", toSearch))

	services, _ := h.clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
	for _, service := range services.Items {
		for serviceAnnotation, serviceAnnotationValue := range service.Annotations {

			if serviceAnnotation == AnnotationNcpSnatPool && serviceAnnotationValue == toSearch {
				h.logger.Debug("Found existing service with same value", zap.String("service", service.Name))
				return &admissionv1.AdmissionResponse{
					UID:     ar.Request.UID,
					Allowed: false,
					Result:  &metav1.Status{Message: fmt.Sprintf("Service %s in namespace %s already has the same value for annotation \"%s\": %s", service.Name, service.Namespace, AnnotationNcpSnatPool, toSearch)},
				}
			}
		}
	}
	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}
