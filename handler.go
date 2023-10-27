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

var serviceRessource = metav1.GroupVersionResource{Version: "v1", Resource: "services"}

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

// validate is the actual admission handler function.
// It checks if the request is for a service and if the service has the
// annotation "ncp/snat_pool" set.
// If the annotation is not set, the request is admitted.
// If the annotation is set and no other service with the same value exists,
// the request is admitted.
// TODO: Add AuditAnnotations to the response.
func (h *admitHandlerV1) validate(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	l := h.logger.With(
		zap.String("namespace", ar.Request.Namespace),
		zap.String("kind", ar.Request.Kind.Kind),
		zap.String("name", ar.Request.Name),
		zap.String("operation", string(ar.Request.Operation)),
		zap.String("uid", string(ar.Request.UID)),
		zap.String("annotation", AnnotationNcpSnatPool))

	defer l.Sync()

	l.Info("Validating request")

	l.Debug("Request context",
		zap.String("group", ar.Request.Kind.Group),
		zap.String("version", ar.Request.Kind.Version),
		zap.String("resource", ar.Request.Resource.String()))

	if ar.Request.Resource != serviceRessource {
		l.Warn("Request is not for a (supported) service", zap.String("group", ar.Request.Kind.Group), zap.String("version", ar.Request.Kind.Version), zap.String("kind", ar.Request.Kind.Kind))
		return &admissionv1.AdmissionResponse{
			UID:      ar.Request.UID,
			Allowed:  true,
			Warnings: []string{"unik: Request does not contain a supported service"},
		}
	}

	svc := corev1.Service{}

	// Maybe the return values should be used, but it seems redundant to me
	// at the moment.
	_, _, _ = deserializer.Decode(ar.Request.Object.Raw, nil, &svc)

	toSearch, present := svc.Annotations[AnnotationNcpSnatPool]

	if !present {
		defer l.Info("Admitted request", zap.String("reason", "annotation not present"))
		return &admissionv1.AdmissionResponse{
			UID:     ar.Request.UID,
			Allowed: true,
		}
	}

	l.Info("Found annotation, checking existing services", zap.String("value", toSearch))

	services, _ := h.clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
	for _, service := range services.Items {

		// TODO: What happens if the service changes the annotation to one that is already
		// used by a different service?
		if service.Namespace == ar.Request.Namespace && service.Name == ar.Request.Name {
			continue
		}
		for serviceAnnotation, serviceAnnotationValue := range service.Annotations {
			if serviceAnnotation == AnnotationNcpSnatPool && serviceAnnotationValue == toSearch {
				l.Info("Denied request", zap.String("reason", "annotation already present"), zap.String("service", fmt.Sprintf("%s/%s", service.Namespace, service.Name)))
				return &admissionv1.AdmissionResponse{
					UID:     ar.Request.UID,
					Allowed: false,
					Result:  &metav1.Status{Message: fmt.Sprintf("Service %s/%s already has the same value for annotation \"%s\": \"%s\"", service.Namespace, service.Name, AnnotationNcpSnatPool, toSearch)},
				}
			}
		}
		defer l.Info("Admitted request", zap.String("reason", "annotation value unique"))
	}
	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}
