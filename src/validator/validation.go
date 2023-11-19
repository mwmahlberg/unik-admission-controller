/*
 *     validation.go is part of github.com/unik-k8s/admission-controller.
 *
 *     Copyright 2023 Markus W Mahlberg <07.federkleid-nagelhaut@icloud.com>
 *
 *     Licensed under the Apache License, Version 2.0 (the "License");
 *     you may not use this file except in compliance with the License.
 *     You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 *     Unless required by applicable law or agreed to in writing, software
 *     distributed under the License is distributed on an "AS IS" BASIS,
 *     WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *     See the License for the specific language governing permissions and
 *     limitations under the License.
 *
 */

package validator

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
)

const (
	admittedRequest string = "Admitted request"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecFactory  = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecFactory.UniversalDeserializer()
)

func init() {
	// See https://github.com/kubernetes-sigs/controller-runtime/issues/1161
	admissionv1.AddToScheme(runtimeScheme)

}

type ValidationHandlerV1 interface {
	ValidateBytes(data []byte) *admissionv1.AdmissionReview
	Validate(admissionv1.AdmissionReview) *admissionv1.AdmissionResponse
}

// AdmitHandlerV1 is a wrapper around an admission handler function.
// Using it allows us to implement various versions of the admission API.
type AdmitHandlerV1 struct {
	clientset kubernetes.Interface
	logger    *zap.Logger
	lock      sync.Mutex
	unique    *UniqueList
}

var serviceRessource = metav1.GroupVersionResource{Version: "v1", Resource: "services"}

type ValidationHandlerOption func(*AdmitHandlerV1) error

func WithLogger(logger *zap.Logger) ValidationHandlerOption {
	return func(h *AdmitHandlerV1) error {
		if logger == nil {
			return errors.New("logger is nil")
		}
		h.logger = logger
		return nil
	}
}

func WithClientset(clientset kubernetes.Interface) ValidationHandlerOption {
	return func(h *AdmitHandlerV1) error {
		if clientset == nil {
			return errors.New("clientset is nil")
		}
		h.clientset = clientset
		return nil
	}
}

func WithUniqueList(unique *UniqueList) ValidationHandlerOption {
	return func(h *AdmitHandlerV1) error {
		if unique == nil {
			return errors.New("unique is nil")
		}
		h.unique = unique
		return nil
	}
}

func NewValidationHandlerV1(options ...ValidationHandlerOption) (*AdmitHandlerV1, error) {
	h := &AdmitHandlerV1{}
	var err error
	for _, option := range options {
		if err = option(h); err != nil {
			return nil, fmt.Errorf("error while applying option: %w", err)
		}
	}

	return h, nil
}

func (h *AdmitHandlerV1) ValidateBytes(data []byte) *admissionv1.AdmissionReview {
	h.lock.Lock()
	defer h.lock.Unlock()
	rto, gvk, err := deserializer.Decode(data, nil, nil)
	if err != nil {
		panic(errors.New("failed to decode request object"))
	}

	if gvk.Group != admissionv1.GroupName || gvk.Version != "v1" || gvk.Kind != "AdmissionReview" {
		panic(errors.New("unexpected group, version or kind"))
	}
	review, ok := rto.(*admissionv1.AdmissionReview)
	if !ok {
		panic(errors.New("expected v1.AdmissionReview"))

	}
	review.Response = h.Validate(*review)

	return review
}

// validate is the actual admission handler function.
// It checks if the request is for a service and if the service has the
// annotation "ncp/snat_pool" set.
// If the annotation is not set, the request is admitted.
// If the annotation is set and no other service with the same value exists,
// the request is admitted.
// TODO: Add AuditAnnotations to the response.
func (h *AdmitHandlerV1) Validate(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	l := h.logger.With(
		zap.String("request.namespace", ar.Request.Namespace),
		zap.String("request.kind", ar.Request.Kind.Kind),
		zap.String("request.name", ar.Request.Name),
		zap.String("request.operation", string(ar.Request.Operation)),
		zap.String("request.uid", string(ar.Request.UID)))

	defer l.Sync()

	l.Info("Validating request")

	l.Debug("Request context",
		zap.String("request.group", ar.Request.Kind.Group),
		zap.String("request.version", ar.Request.Kind.Version),
		zap.String("request.resource", ar.Request.Resource.String()))

	if ar.Request.Resource != serviceRessource {
		l.Warn("Request is not for a (supported) service", zap.String("group", ar.Request.Kind.Group), zap.String("version", ar.Request.Kind.Version), zap.String("kind", ar.Request.Kind.Kind))
		return &admissionv1.AdmissionResponse{
			UID:      ar.Request.UID,
			Allowed:  true,
			Warnings: []string{"unik: Request does not contain a supported service"},
		}
	}

	svcToCheck := corev1.Service{}

	// Maybe the return values should be used, but it seems redundant to me
	// at the moment.
	_, _, err := deserializer.Decode(ar.Request.Object.Raw, nil, &svcToCheck)

	if err != nil {
		l.DPanic("Failed to decode request object", zap.Error(err))
	}

	response := &admissionv1.AdmissionResponse{
		UID: ar.Request.UID,
	}
	if h.unique.HasDuplicate() {
		l.Warn("Configuration has annotations protected in cluster scope and in namespace scope")
		response.Warnings = []string{"unik: Configuration has annotations protected in cluster scope and in namespace scope"}
	}

	if !h.unique.HasProtectedAnnotations(maps.Keys(svcToCheck.Annotations)) {
		l.Debug("No protected annotations")
		defer l.Info(admittedRequest, zap.String("reason", "no protected annotations"))
		response.Allowed = true
		return response
	}

	// We only want to check if the annotation is marked as unique in the
	// namespace of the service or in the cluster scope.
	toCheck := h.unique.Filter(Namespace(svcToCheck.Namespace), maps.Keys(svcToCheck.Annotations))

	for _, scope := range toCheck.Scopes() {
		if !h.unique.HasProtectedInNamespace(scope, svcToCheck.Annotations) {
			l.Debug("No protected annotations in scope", zap.String("scope", string(scope)))
			continue
		}
		ns := string(scope)
		if scope == ClusterScope {
			ns = ""
		}
		l.Debug("Checking services in scope", zap.String("scope", string(scope)), zap.String("namespace", ns))
		servicesInScope, _ := h.clientset.CoreV1().Services(ns).List(context.TODO(), metav1.ListOptions{})
		for _, svcInScope := range servicesInScope.Items {
			l.Debug("Checking service", zap.String("service", svcInScope.Name), zap.String("namespace", svcInScope.Namespace))
			// We do not need to check the service to be admitted.
			// We can do this because even when the service is changed,
			// the value of the annotation will be checked against the
			// values of the other services.
			if svcInScope.Name == svcToCheck.Name && svcInScope.Namespace == svcToCheck.Namespace {
				continue
			}

			for annotationKey, annotationValue := range svcToCheck.Annotations {
				l.Debug("Checking annotation", zap.String("service", svcInScope.Name), zap.String("namespace", svcInScope.Namespace), zap.String("annotation", string(annotationKey)))
				// Skip if the service from the scope does not have the
				// annotation we want to check.
				if _, ok := svcInScope.Annotations[annotationKey]; !ok {
					l.Debug("Service does not have annotation",
						zap.String("service", svcInScope.Name),
						zap.String("annotation", string(annotationKey)),
						zap.String("value", string(annotationValue)))
					continue
				}

				if svcInScope.Annotations[annotationKey] == svcToCheck.Annotations[annotationKey] {
					l.Warn("Denied request",
						zap.String("reason", "service exists with the same value for annotation"),
						zap.String("namespace", svcInScope.Namespace),
						zap.String("service", svcInScope.Name),
						zap.String("annotation", string(annotationKey)),
						zap.String("value", string(annotationValue)))

					response.Allowed = false
					response.Result = &metav1.Status{
						Message: fmt.Sprintf("Service %s/%s already has the same value for annotation \"%s\": %s", svcInScope.Namespace, svcInScope.Name, annotationKey, string(annotationValue)),
					}
					return response
				}
			}
		}
	}
	l.Info(admittedRequest, zap.String("reason", "no duplicate annotations"))
	response.Allowed = true
	return response
}
