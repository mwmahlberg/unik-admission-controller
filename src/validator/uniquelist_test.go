package validator_test

import (
	"testing"

	"github.com/mwmahlberg/unik-admission-controller/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type UniqueListSuite struct {
	suite.Suite
}

func (s *UniqueListSuite) TestHasNamespace() {
	testCases := []struct {
		desc     string
		list     *validator.UniqueList
		lookup   validator.Namespace
		expected bool
	}{
		{
			desc: "ClusterScope",
			list: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					validator.ClusterScope: nil,
				},
			},
			lookup:   validator.ClusterScope,
			expected: true,
		},
		{
			desc: "Namespace",
			list: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					"test": nil,
				},
			},
			lookup:   "test",
			expected: true,
		},
		{
			desc: "Not found",
			list: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					"test": nil,
				},
			},
			lookup:   "notfound",
			expected: false,
		},
	}
	for _, tC := range testCases {
		s.T().Run(tC.desc, func(t *testing.T) {
			assert.Equal(t, tC.expected, tC.list.HasNamespace(tC.lookup))
		})
	}
}

func (s *UniqueListSuite) TestProtectedInNamespace() {
	testCases := []struct {
		desc      string
		list      *validator.UniqueList
		namespace validator.Namespace
		lookup    validator.Annotation
		expected  bool
	}{
		{
			desc: "Found",
			list: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					"test": {
						validator.AnnotationNcpSnatPool,
					},
				},
			},
			namespace: "test",
			lookup:    validator.AnnotationNcpSnatPool,
			expected:  true,
		},
		{
			desc: "Not found",
			list: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					"test": {
						"something",
					},
				},
			},
			namespace: "test",
			lookup:    validator.AnnotationNcpSnatPool,
			expected:  false,
		},
		{
			desc: "Not found in namespace",
			list: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					"test": {
						validator.AnnotationNcpSnatPool,
					},
				},
			},
			namespace: "other",
			lookup:    validator.AnnotationNcpSnatPool,
			expected:  false,
		},
	}
	for _, tC := range testCases {
		s.T().Run(tC.desc, func(t *testing.T) {
			assert.Equal(t, tC.expected, tC.list.ProtectedInNamespace(tC.namespace, tC.lookup))
		})
	}
}

func (s *UniqueListSuite) TestFilter() {
	testCases := []struct {
		desc        string
		protected   *validator.UniqueList
		lookup      validator.Namespace
		annotations []validator.Annotation
		expected    *validator.UniqueList
	}{
		{
			desc: "",
			protected: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					validator.ClusterScope: {
						validator.AnnotationNcpSnatPool,
					},
					"test": {
						"foo",
					},
					"other": {
						"bar",
					},
				},
			},
			lookup: validator.ClusterScope,
			annotations: []validator.Annotation{
				validator.AnnotationNcpSnatPool,
			},
			expected: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					validator.ClusterScope: {
						validator.AnnotationNcpSnatPool,
					},
				}},
		},
	}
	for _, tC := range testCases {
		s.T().Run(tC.desc, func(t *testing.T) {

		})
	}

}

func (s *UniqueListSuite) TestProtectedInCluster() {
	testCases := []struct {
		desc     string
		list     *validator.UniqueList
		lookup   validator.Annotation
		expected bool
	}{
		{
			desc: "Found",
			list: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					validator.ClusterScope: {
						validator.AnnotationNcpSnatPool,
					},
				},
			},
			lookup:   validator.AnnotationNcpSnatPool,
			expected: true,
		},
		{
			desc: "Not found",
			list: &validator.UniqueList{
				Annotations: map[validator.Namespace][]validator.Annotation{
					validator.ClusterScope: {
						"something",
					},
				},
			},
			lookup:   validator.AnnotationNcpSnatPool,
			expected: false,
		},
	}
	for _, tC := range testCases {
		s.T().Run(tC.desc, func(t *testing.T) {
			assert.Equal(t, tC.expected, tC.list.ProtectedInCluster(tC.lookup))
		})
	}
}

func TestUniqueList(t *testing.T) {
	suite.Run(t, new(UniqueListSuite))
}
