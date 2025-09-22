package nodepools

import (
	"sort"
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestLabelsTaintsAnnotationsToRemove(t *testing.T) {
	type args struct {
		current []*spec.NodePool
		desired []*spec.NodePool
	}
	tests := []struct {
		Name string
		Args args
		Want LabelsTaintsAnnotationsData
	}{
		{
			Name: "empty-ok",
			Args: args{
				current: nil,
				desired: nil,
			},
			Want: LabelsTaintsAnnotationsData{
				LabelKeys:      map[string][]string{},
				AnnotationKeys: map[string][]string{},
				TaintKeys:      map[string][]*spec.Taint{},
			},
		},
		{
			Name: "Labels mismatch",
			Args: args{
				current: []*spec.NodePool{
					{
						Name: "np1",
						Labels: map[string]string{
							"test-label-1": "1",
							"test-label-2": "2",
							"test-label-3": "3",
						},
					},
					{
						Name: "np2",
						Labels: map[string]string{
							"test-label-1": "1",
							"test-label-3": "3",
						},
					},
					{
						Name: "np3",
						Labels: map[string]string{
							"test-label-1": "4",
							"test-label-3": "5",
						},
					},
				},
				desired: []*spec.NodePool{
					{
						Name: "np1",
						Labels: map[string]string{
							"test-label-2": "2",
							"test-label-3": "3",
						},
					},
					{
						Name: "np2",
						Labels: map[string]string{
							"test-label-3": "3",
						},
					},
					{
						Name:   "np3",
						Labels: map[string]string{},
					},
				},
			},
			Want: LabelsTaintsAnnotationsData{
				LabelKeys: map[string][]string{
					"np1": {
						"test-label-1",
					},
					"np2": {
						"test-label-1",
					},
					"np3": {
						"test-label-1",
						"test-label-3",
					},
				},
				AnnotationKeys: map[string][]string{},
				TaintKeys:      map[string][]*spec.Taint{},
			},
		},
		{
			Name: "Annotations/Labels mismatch",
			Args: args{
				current: []*spec.NodePool{
					{
						Name: "np1",
						Labels: map[string]string{
							"test-label-1": "1",
							"test-label-2": "2",
							"test-label-3": "3",
						},
						Annotations: map[string]string{
							"test-annotations-1": "1",
							"test-annotations-2": "2",
							"test-annotations-3": "3",
						},
					},
					{
						Name: "np2",
						Labels: map[string]string{
							"test-label-1": "1",
							"test-label-3": "3",
						},
						Annotations: map[string]string{
							"test-annotations-1": "1",
							"test-annotations-3": "3",
						},
					},
					{
						Name: "np3",
						Labels: map[string]string{
							"test-label-1": "4",
							"test-label-3": "5",
						},
						Annotations: map[string]string{
							"test-annotations-1": "4",
							"test-annotations-3": "5",
						},
					},
				},
				desired: []*spec.NodePool{
					{
						Name: "np1",
						Labels: map[string]string{
							"test-label-2": "2",
							"test-label-3": "3",
						},
						Annotations: map[string]string{
							"test-annotations-2": "2",
							"test-annotations-3": "3",
						},
					},
					{
						Name: "np2",
						Labels: map[string]string{
							"test-label-3": "3",
						},
						Annotations: map[string]string{
							"test-annotations-3": "3",
						},
					},
					{
						Name:        "np3",
						Labels:      map[string]string{},
						Annotations: map[string]string{},
					},
				},
			},
			Want: LabelsTaintsAnnotationsData{
				LabelKeys: map[string][]string{
					"np1": {
						"test-label-1",
					},
					"np2": {
						"test-label-1",
					},
					"np3": {
						"test-label-1",
						"test-label-3",
					},
				},
				AnnotationKeys: map[string][]string{
					"np1": {
						"test-annotations-1",
					},
					"np2": {
						"test-annotations-1",
					},
					"np3": {
						"test-annotations-1",
						"test-annotations-3",
					},
				},
				TaintKeys: map[string][]*spec.Taint{},
			},
		},
		{
			Name: "Taints mismatch",
			Args: args{
				current: []*spec.NodePool{
					{
						Name: "np1",
						Taints: []*spec.Taint{
							{
								Key:    "Taint-1",
								Value:  "1",
								Effect: "NoSchedule",
							},
							{
								Key:    "Taint-2",
								Value:  "2",
								Effect: "NoSchedule",
							},
						},
					},
					{
						Name: "np2",
						Taints: []*spec.Taint{
							{
								Key:    "Taint-2",
								Value:  "2",
								Effect: "NoSchedule",
							},
							{
								Key:    "Taint-3",
								Value:  "3",
								Effect: "NoSchedule",
							},
							{
								Key:    "Taint-4",
								Value:  "4",
								Effect: "NoSchedule",
							},
						},
					},
					{
						Name: "np3",
						Taints: []*spec.Taint{
							{
								Key:    "Taint-2",
								Value:  "2",
								Effect: "NoSchedule",
							},
							{
								Key:    "Taint-4",
								Value:  "4",
								Effect: "NoSchedule",
							},
						},
					},
				},
				desired: []*spec.NodePool{
					{
						Name: "np1",
						Taints: []*spec.Taint{
							{
								Key:    "Taint-1",
								Value:  "1",
								Effect: "NoSchedule",
							},
						},
					},
					{
						Name: "np2",
						Taints: []*spec.Taint{
							{
								Key:    "Taint-2",
								Value:  "2",
								Effect: "NoSchedule",
							},
							{
								Key:    "Taint-4",
								Value:  "4",
								Effect: "NoSchedule",
							},
						},
					},
					{
						Name:   "np3",
						Taints: nil,
					},
				},
			},
			Want: LabelsTaintsAnnotationsData{
				LabelKeys:      map[string][]string{},
				AnnotationKeys: map[string][]string{},
				TaintKeys: map[string][]*spec.Taint{
					"np1": {{
						Key:    "Taint-2",
						Value:  "2",
						Effect: "NoSchedule",
					}},
					"np2": {{
						Key:    "Taint-3",
						Value:  "3",
						Effect: "NoSchedule",
					}},
					"np3": {
						{
							Key:    "Taint-2",
							Value:  "2",
							Effect: "NoSchedule",
						},
						{
							Key:    "Taint-4",
							Value:  "4",
							Effect: "NoSchedule",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got := LabelsTaintsAnnotationsDiff(tt.Args.current, tt.Args.desired)

			// sort the output to be same every call.

			for _, v := range got.LabelKeys {
				sort.Strings(v)
			}
			for _, v := range got.AnnotationKeys {
				sort.Strings(v)
			}
			for _, v := range got.TaintKeys {
				sort.Slice(v, func(i, j int) bool { return v[i].Key < v[j].Key })
			}

			if diff := cmp.Diff(got, tt.Want, protocmp.Transform()); diff != "" {
				t.Fatalf("labelsTaintsAnnotationsToRemove(%s) = %s", tt.Name, diff)
			}
		})
	}
}
