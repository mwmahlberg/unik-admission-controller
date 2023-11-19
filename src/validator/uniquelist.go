package validator

import (
	"slices"
	"sync"

	"golang.org/x/exp/maps"
)

type Namespace string

func (n Namespace) String() string {
	return string(n)
}

type Annotation string

func (a Annotation) String() string {
	return string(a)
}

const (
	AnnotationNcpSnatPool Annotation = "ncp/snat_pool"
	ClusterScope          Namespace  = "*"
)

type UniqueList struct {
	sync.RWMutex
	Annotations map[Namespace][]Annotation `json:"annotations"`
}

func (s *UniqueList) HasNamespace(namespace Namespace) bool {
	s.RLock()
	defer s.RUnlock()

	_, ok := s.Annotations[namespace]
	return ok
}

// ProtectedInNamespace checks if the given annotation is protected in the given namespace.
func (s *UniqueList) ProtectedInNamespace(namespace Namespace, annotation Annotation) bool {
	s.RLock()
	defer s.RUnlock()

	if !s.HasNamespace(namespace) {
		return false
	}

	return slices.Contains(s.Annotations[namespace], annotation)

}

// Filter returns a new UniqueList with only the given namespace, if it exists, the cluster scope and the protected annotations for the given set of annotations.
func (s *UniqueList) Filter(namespace Namespace, serviceAnnotations []string) *UniqueList {
	s.RLock()
	defer s.RUnlock()

	if !s.HasNamespace(namespace) && !s.HasNamespace(ClusterScope) {
		return nil
	}

	filtered := &UniqueList{
		Annotations: map[Namespace][]Annotation{},
	}
	for _, annotation := range serviceAnnotations {
		if s.ProtectedInNamespace(namespace, Annotation(annotation)) {
			filtered.Annotations[namespace] = append(filtered.Annotations[namespace], Annotation(annotation))
		}
		if s.ProtectedInCluster(Annotation(annotation)) {
			filtered.Annotations[ClusterScope] = append(filtered.Annotations[ClusterScope], Annotation(annotation))
		}
	}
	return filtered
}

// Scopes returns all scopes in which annotations are protected.
func (s *UniqueList) Scopes() []Namespace {
	s.RLock()
	defer s.RUnlock()
	return maps.Keys(s.Annotations)
}

// HasDuplicate checks whether there are annotations protected both in Namespace and ClusterScope.
func (s *UniqueList) HasDuplicate() bool {
	s.RLock()
	defer s.RUnlock()

	for namespace, annotations := range s.Annotations {
		if namespace == ClusterScope {
			continue
		}
		for _, a := range annotations {
			if s.ProtectedInCluster(a) {
				return true
			}
		}
	}

	return false
}

// ProtectedInCluster checks if the given annotation is protected in cluster scope.
func (s *UniqueList) ProtectedInCluster(annotation Annotation) bool {
	s.RLock()
	defer s.RUnlock()
	return s.ProtectedInNamespace(ClusterScope, annotation)
}

// ProtectedInAnyNamespace checks if the given annotation is protected in any namespace except cluster scope.
func (s *UniqueList) ProtectedInAnyNamespace(annotation Annotation) bool {
	s.RLock()
	defer s.RUnlock()

	for namespace, annotations := range s.Annotations {
		if namespace == ClusterScope {
			continue
		}
		if slices.Contains(annotations, annotation) {
			return true
		}
	}

	return false
}

// HasProtectedInNamespace checks if one of the given annotations is protected in the given namespace.
func (s *UniqueList) HasProtectedInNamespace(namespace Namespace, annotations map[string]string) bool {
	if !s.HasNamespace(namespace) {
		return false
	}
	for _, annotation := range maps.Keys(annotations) {
		if slices.Contains(s.Annotations[namespace], Annotation(annotation)) {
			return true
		}
	}
	return false
}

// IsProtected checks if the given annotation is protected in any namespace
// including cluster scope.
func (s *UniqueList) IsProtected(annotation Annotation) bool {

	s.RLock()
	defer s.RUnlock()

	return s.ProtectedInCluster(annotation) || s.ProtectedInAnyNamespace(annotation)

}

// HasProtectedAnnotations checks if one of the given annotations is protected in any namespace.
func (s *UniqueList) HasProtectedAnnotations(serviceAnnotations []string) bool {
	s.RLock()
	defer s.RUnlock()

	for _, annotation := range serviceAnnotations {
		if s.IsProtected(Annotation(annotation)) {
			return true
		}
	}

	return false
}
