/*
Copyright 2023 Lixoi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var torchainlog = logf.Log.WithName("torchain-resource")

func (r *TorChain) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-torchain-gate-way-v1alpha1-torchain,mutating=true,failurePolicy=fail,sideEffects=None,groups=torchain.gate.way,resources=torchains,verbs=create;update,versions=v1alpha1,name=mtorchain.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &TorChain{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *TorChain) Default() {
	torchainlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-torchain-gate-way-v1alpha1-torchain,mutating=false,failurePolicy=fail,sideEffects=None,groups=torchain.gate.way,resources=torchains,verbs=create;update,versions=v1alpha1,name=vtorchain.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &TorChain{}

func (r *TorChain) validateDeploymens() error {
	var allErrs field.ErrorList
	if r.Spec.Deployments != r.Spec.LengthChain {
		fldPath := field.NewPath("spec").Child("deployments")
		allErrs = append(allErrs, field.Invalid(fldPath, strconv.Itoa(r.Spec.Deployments), "Count of deployments isn't equal length of chain"))
	}
	for i, v := range r.Status.Nodes {
		if v.BadConnectsCounter > 10 {
			fldPath := field.NewPath("status").Child("BadConnectsCounter")
			allErrs = append(allErrs, field.Invalid(fldPath, strconv.Itoa(i), "Node of chain is bad connect"))
		}
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "torchain.gate.way", Kind: "TorChain"}, r.Name, allErrs)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *TorChain) ValidateCreate() error {
	torchainlog.Info("validate create", "name", r.Name)
	// TODO(user): fill in your validation logic upon object creation.
	return r.validateDeploymens()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *TorChain) ValidateUpdate(old runtime.Object) error {
	torchainlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return r.validateDeploymens()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *TorChain) ValidateDelete() error {
	torchainlog.Info("validate delete", "name", r.Name)
	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
