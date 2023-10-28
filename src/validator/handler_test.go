/* 
 *     handler_test.go is part of unik-k8s.
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var defaultService = []byte(
	`{
	"apiVersion": "v1",
	"kind": "Service",
	"metadata": {
		"annotations": {
			"ncp/snat_pool": "test"
		},
		"name": "test",
		"namespace": "default"
	}
}`)

var defaultServiceWithoutAnnotation = []byte(
	`{
	"apiVersion": "v1",
	"kind": "Service",
	"metadata": {
		"name": "test",
		"namespace": "default"
	}
}`)

var ar = admissionv1.AdmissionReview{
	Request: &admissionv1.AdmissionRequest{
		UID: "test",
		Kind: metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Service",
		},
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "services",
		},
		Name:      "test",
		Namespace: "default",
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: defaultService,
		},
	},
}

var arWithoutAnnotation = admissionv1.AdmissionReview{
	Request: &admissionv1.AdmissionRequest{
		UID: "test",
		Kind: metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Service",
		},
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "services",
		},
		Name:      "test",
		Namespace: "default",
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: defaultServiceWithoutAnnotation,
		},
	},
}

var serviceNoAnnotation = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name:        "no-annotation",
		Namespace:   "default",
		Annotations: map[string]string{},
	},
}

var serviceWithAnnotationOtherValue = corev1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name:        "with-annotation",
		Namespace:   "default",
		Annotations: map[string]string{AnnotationNcpSnatPool: "other"},
	},
}

type HandlerSuite struct {
	suite.Suite
}

func (s *HandlerSuite) TestHandlerOld() {
	tc := testclient.NewSimpleClientset()
	tc.Fake.PrependReactor("list", "services",
		func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, &corev1.ServiceList{}, nil
		})
	h, err := NewValidationHandlerV1(WithLogger(zaptest.NewLogger(s.T())), WithClientset(tc))
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), h)
	response := h.Validate(ar)
	assert.NotNil(s.T(), response)
}

func emptyServiceList(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
	return true, &corev1.ServiceList{}, nil
}

func listWithService(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
	return true, &corev1.ServiceList{
		Items: []corev1.Service{
			serviceNoAnnotation,
		},
	}, nil
}

func listWithServiceAndAnnotation(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
	return true, &corev1.ServiceList{
		Items: []corev1.Service{
			serviceWithAnnotationOtherValue,
		},
	}, nil
}

func (s *HandlerSuite) TestHandlerAdmission() {

	testCases := []struct {
		desc    string
		reactor k8stesting.ReactionFunc
		ar      admissionv1.AdmissionReview
	}{
		{
			desc:    "empty service list",
			reactor: emptyServiceList,
			ar:      ar,
		},
		{
			desc:    "list with service, no annotation",
			reactor: listWithService,
			ar:      ar,
		},
		{
			desc:    "list with service and annotation, different value",
			reactor: listWithServiceAndAnnotation,
			ar:      ar,
		},
		{
			desc:    "request without annotation",
			reactor: emptyServiceList,
			ar:      arWithoutAnnotation,
		},
	}
	for _, tC := range testCases {

		s.T().Run(tC.desc, func(t *testing.T) {

			tc := testclient.NewSimpleClientset()
			tc.Fake.PrependReactor("list", "services", tC.reactor)

			h, err := NewValidationHandlerV1(WithLogger(zaptest.NewLogger(t)), WithClientset(tc))
			assert.NoError(t, err)
			assert.NotNil(t, h)

			response := h.Validate(tC.ar)
			assert.NotNil(t, response)
			assert.True(t, response.Allowed)
		})
	}
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerSuite))
}
