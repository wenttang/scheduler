package judgment

import (
	"fmt"
	"strings"

	"github.com/wenttang/workflow/pkg/apis/v1alpha1"
)

type Judgment interface {
	GetTag() string
	Adjudication(intput *v1alpha1.AnyString, values []*v1alpha1.AnyString) bool
}

var warehouse = struct {
	store map[string]Judgment
}{}

func init() {
	judgment := []Judgment{
		&in{},
		&notIn{},
		&lt{},
		&lte{},
		&gt{},
		&gte{},
	}

	warehouse.store = map[string]Judgment{}
	for _, elem := range judgment {
		warehouse.store[elem.GetTag()] = elem
	}
}

func Adjudication(op string, intput *v1alpha1.AnyString, values []*v1alpha1.AnyString) (bool, error) {
	upper := strings.ToUpper(op)
	judgment, ok := warehouse.store[upper]
	if !ok {
		return false, fmt.Errorf("unexpected operator symbol [%s]", op)
	}

	return judgment.Adjudication(intput, values), nil
}

type in struct{}

func (in *in) GetTag() string {
	return "IN"
}

func (in *in) Adjudication(intput *v1alpha1.AnyString, values []*v1alpha1.AnyString) bool {
	val := *intput.GetString()
	for _, elem := range values {
		if *elem.GetString() == val {
			return true
		}
	}
	return false
}

type notIn struct{}

func (notin *notIn) GetTag() string {
	return "NOTIN"
}

func (notin *notIn) Adjudication(intput *v1alpha1.AnyString, values []*v1alpha1.AnyString) bool {
	val := intput.GetString()
	for _, elem := range values {
		if *elem.GetString() == *val {
			return false
		}
	}
	return true
}

type lt struct{}

func (lt *lt) GetTag() string {
	return "LT"
}

func (lt *lt) Adjudication(intput *v1alpha1.AnyString, values []*v1alpha1.AnyString) bool {
	val := intput.GetString()
	for _, elem := range values {
		if !jlt(*val, *elem.GetString()) {
			return false
		}
	}

	return true
}

type lte struct{}

func (lte *lte) GetTag() string {
	return "LTE"
}

func (lte *lte) Adjudication(intput *v1alpha1.AnyString, values []*v1alpha1.AnyString) bool {
	val := intput.GetString()
	for _, elem := range values {
		if !jlte(*val, *elem.GetString()) {
			return false
		}
	}

	return true
}

type gt struct{}

func (gt *gt) GetTag() string {
	return "GT"
}

func (gt *gt) Adjudication(intput *v1alpha1.AnyString, values []*v1alpha1.AnyString) bool {
	val := intput.GetString()
	for _, elem := range values {
		if jlt(*val, *elem.GetString()) {
			return false
		}
	}

	return true
}

type gte struct{}

func (gte *gte) GetTag() string {
	return "GTE"
}

func (gte *gte) Adjudication(intput *v1alpha1.AnyString, values []*v1alpha1.AnyString) bool {
	val := intput.GetString()
	for _, elem := range values {
		if jlte(*val, *elem.GetString()) {
			return false
		}
	}

	return true
}

type Number interface {
	int | int32 | int64 | float32 | float64 | string
}

func jlt[T Number](p1, p2 T) bool {
	return p1 < p2
}

func jlte[T Number](p1, p2 T) bool {
	return p1 <= p2
}
