package controller

import (
	"github.com/bszeti/battlefield-operator/pkg/controller/battlefield"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, battlefield.Add)
}
