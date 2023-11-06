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

package controllers

import (
	"context"

	netattachdef "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	torchainv1alpha1 "github.com/operator-framework/operator-sdk/api/v1alpha1"
)

// TorChainReconciler reconciles a TorChain object
type TorChainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Описание разрешений (будут транслированы в роли)
//+kubebuilder:rbac:groups=torchain.gate.way,resources=torchains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=torchain.gate.way,resources=torchains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=torchain.gate.way,resources=torchains/finalizers,verbs=update

//+kubebuilder:rbac:groups=apps/v1,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps/v1,resources=pods,verbs=get;list;patch
//+kubebuilder:rbac:groups=v1,resources=secrets,verbs=get;list;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TorChain object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *TorChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	multusParams := make([]netattachdef.NetworkAttachmentDefinition, 1, 3)
	// проверяем инициализацию интерфейсов для цепочки
	multusInterface := &netattachdef.NetworkAttachmentDefinition{}
	err := r.Get(ctx, req.NamespacedName, multusInterface)
	if err != nil {
		if errors.IsNotFound(err) {
			// не были заданы интерфейсы, цепрочка не может быть построена
			return ctrl.Result{}, nil
		}
		// ошибка получения ресурса
		return ctrl.Result{}, err
	}
	multusParams = append(multusParams, *multusInterface)
	// считываем имена интейрффейсов

	chainNode := &torchainv1alpha1.TorChain{}

	// получаем список узлов цепочки
	nodesList := &torchainv1alpha1.TorChainList{}
	listOpts := []client.ListOption{
		client.InNamespace(chainNode.Namespace),
		client.MatchingLabels{"instance": chainNode.Name},
		//client.MatchingFields{"status.phase": "Running"},
	}
	if err = r.List(ctx, nodesList, listOpts...); err != nil {
		return ctrl.Result{}, err
	}

	if len(nodesList.Items) == 0 {
		return ctrl.Result{}, nil // если не задан ни один узел цепочки
	}

	for _, nodeChain := range nodesList.Items {
		_ = nodeChain
	}

	err = r.Get(ctx, types.NamespacedName{Name: chainNode.Name, Namespace: chainNode.Namespace}, chainNode)
	if err != nil {
		if errors.IsNotFound(err) {
			// узел был удален, удалим развертывание
			sts := &appsv1.Deployment{}
			err = r.Get(ctx, types.NamespacedName{Name: chainNode.Name, Namespace: chainNode.Namespace}, sts)
			if err == nil {
				err = r.Delete(ctx, sts)
			}
		}
		// ошибка получения ресурса
		// log.Error(err, "Failed to get HelloApp")
		return ctrl.Result{}, err
	}

	// если deployment не был создан

	return ctrl.Result{}, nil
}

func (r *TorChainReconciler) deployNodeChain() {

}

// SetupWithManager sets up the controller with the Manager.
func (r *TorChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&torchainv1alpha1.TorChain{}).
		// сущности в RC, которыми будем управлять
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
