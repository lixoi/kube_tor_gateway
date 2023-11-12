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
	"sync"
	"time"

	netattachdef "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	torchainv1alpha1 "github.com/operator-framework/operator-sdk/api/v1alpha1"
)

var (
	controllerLog = ctrl.Log.WithName("controller")
)

// TorChainReconciler reconciles a TorChain object
type TorChainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	wg     sync.WaitGroup
	mux    sync.Mutex
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

	// запускаем поток, который будет следить за доступностью серверов в цепочке
	if r.mux.TryLock() == true {
		go r.snifferTorChains(ctx)
	}

	// 0. Если узел был удален (delete), удаляем deployment
	// 1. Получаем деплоймент
	// 2. Если нет деплоймента (create), то:
	// 2.1 проверем наличие NetworkAttachmentDefinition для текущего номера цепочки
	// 2.2 если NetworkAttachmentDefinition не заданы, то error (helm chart не правильно задан)
	// 2.3 если NetworkAttachmentDefinition заданы, то задаем деплоймент с sidecar
	// 3. Если доплоймент задан, то меняем конфигурацию (secret) в ПОДе на новую (для нового сервера),
	//    и меняем статус ПОДа и обновляем deployment

	// 0:
	node := &torchainv1alpha1.TorChain{}
	err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, node)
	if err != nil {
		if errors.IsNotFound(err) {
			// узел был удален, удалим развертывание
			deploy := &appsv1.Deployment{}
			err = r.Get(ctx, types.NamespacedName{Name: node.Name, Namespace: node.Namespace}, deploy)
			if err == nil {
				err = r.Delete(ctx, deploy)
			}
			// и выходим из ф-ции
			return ctrl.Result{}, nil
		}
		// ошибка получения ресурса
		// log.Error(err, "Failed to get HelloApp")
		return ctrl.Result{}, err
	}

	// 1:
	deploy := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: node.Name, Namespace: node.Namespace}, deploy)
	if err != nil {
		// 2:
		if errors.IsNotFound(err) {
			return ctrl.Result{}, r.createDeployment()
		}
	}

	// 3:

	multusParams := make([]netattachdef.NetworkAttachmentDefinition, 1, 3)
	// проверяем инициализацию интерфейсов для цепочки
	multusInterface := &netattachdef.NetworkAttachmentDefinition{}
	err = r.Get(ctx, req.NamespacedName, multusInterface)
	if err != nil {
		if errors.IsNotFound(err) {
			// не были заданы интерфейсы, цепрочка не может быть построена
			return ctrl.Result{}, nil
		}
		// ошибка получения ресурса
		return ctrl.Result{}, nil
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

func (r *TorChainReconciler) createDeployment() error {
	// 2.1

	return nil
}

// обход по списку для запуска потока поиска недоступного узла (updateNodeTorChain):
// 1. попадается узел цепочки в состоянии False. Если namespace пода нет в массиве, то запускаем поток поиска и сохраняем namespace узла в массив.
func (r *TorChainReconciler) snifferTorChains(ctx context.Context) error {

	defer r.mux.Unlock()
	listAllPods := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.MatchingLabels{"instance": "torgateway"},
	}

	controllerLog.Info("func snifferTorChains: start")

	for true {
		if err := r.List(ctx, listAllPods, listOpts...); err != nil {
			return err
		}
		nameSpaces := make([]string, 1, 5)
		for _, pod := range listAllPods.Items {
			isInitialized := false
			controllerLog.Info("func snifferTorChains: range pods")
			// break
			for _, podCondition := range pod.Status.Conditions {
				switch podCondition.Type {
				case "Initialized":
					if podCondition.Status == "True" {
						isInitialized = true
					}
				case "Ready":
					nameSpace := pod.GetNamespace()
					if podCondition.Status == "False" && isInitialized == true && !slices.Contains(nameSpaces, nameSpace) {
						nameSpaces = append(nameSpaces, nameSpace)
						r.wg.Add(1)
						go r.updateNodeTorChain(ctx, nameSpace)
					}
				}
			}
		}
		r.wg.Wait()

		time.Sleep(10 * time.Second)
	}

	return nil
}

// работа потока поиска недоступного узла
// 1. Получаем список всех узлов цепочки из заданного namespace
// 2. В полученном списке ищем наименьший порядковый номер узла цепочки, который в состоянии False
// 3. По параметрам полученного узла цепочки ищем CRD для обновления его конфигурации и целью инициировать событие в Reconcile (Update)
// 4. Определяем таймаут и вычисляем максимальное значение для цикла опроса цепочек
func (r *TorChainReconciler) updateNodeTorChain(ctx context.Context, nameSpace string) {
	defer r.wg.Done()

	listPodsInNameSpace := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(nameSpace),
		client.MatchingLabels{"instance": "torgateway"},
		//client.MatchingFields{"restarts.phase": "1"},
	}
	if err := r.List(ctx, listPodsInNameSpace, listOpts...); err != nil {
		return
	}
	crd_deploy := &torchainv1alpha1.TorChain{}
	for _, pod := range listPodsInNameSpace.Items {
		isInitialized := false
		for _, podCondition := range pod.Status.Conditions {
			switch podCondition.Type {
			case "Initialized":
				if podCondition.Status == "True" {
					isInitialized = true
				}
			case "Ready":
				if podCondition.Status == "True" || isInitialized == false {
					break
				}
				current_crd_deploy := &torchainv1alpha1.TorChain{}
				err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: nameSpace}, current_crd_deploy)
				if err != nil {
					break
				}
				if crd_deploy == nil || current_crd_deploy.Spec.NumberNode < crd_deploy.Spec.NumberNode {
					crd_deploy = current_crd_deploy
				}
			}
		}
	}
	crd_deploy.Status.Connected = false
	crd_deploy.Spec.SwitchServer += 1
	r.Update(ctx, crd_deploy)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TorChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&torchainv1alpha1.TorChain{}).
		// сущности в RCD, которыми будем управлять
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
