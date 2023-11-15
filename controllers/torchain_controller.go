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
	"encoding/json"
	"net"
	"sync"
	"time"

	netattachdef "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type NetDefinitionConfig struct {
	CniVersion string `json:"cniVersion"` //nolint:all
	Plugins    []NetDefinitionParams
	NetDefinitionParams
}

type NetDefinitionParams struct {
	Type             string
	Backend          string
	Bridge           string
	IsGateway        bool `json:"isGateway"`        //nolint:all
	IsDefaultGateway bool `json:"isDefaultGateway"` //nolint:all
	IpMasq           bool `json:"ipMasq"`           //nolint:all
	Master           string
	Mode             string
	Ipam             struct {
		Type       string
		Subnet     string
		RangeStart string `json:"rangeStart"` //nolint:all
		RangeEnd   string `json:"rangeEnd"`   //nolint:all
		GateWay    string `json:"gateway"`    //nolint:all
		Routes     []struct {
			Dst string
		}
	}
}

type NetDeploymentAnnotation struct {
	Name      string   `json:"name"`
	Interface string   `json:"interface"`
	Ips       []string `json:"ips"`
}

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

	node := &torchainv1alpha1.TorChain{}
	err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, node)
	// 0: (delete)
	if err != nil {
		if errors.IsNotFound(err) {
			// узел был удален, удалим deployment
			deploy := &appsv1.Deployment{}
			err = r.Get(ctx, types.NamespacedName{Name: node.Name, Namespace: node.Namespace}, deploy)
			if err == nil {
				err = r.Delete(ctx, deploy)
			}
			// и выходим из ф-ции
			return ctrl.Result{}, nil
		}
		// ошибка получения ресурса
		return ctrl.Result{}, err
	}
	// 1:
	deploy := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: node.Name, Namespace: node.Namespace}, deploy)
	if err != nil {
		// 2: (create)
		if errors.IsNotFound(err) {
			return ctrl.Result{}, r.createDeployment(ctx, node)
		}
	}

	// 3:
	secret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, secret)

	return ctrl.Result{}, nil
}

func (r *TorChainReconciler) createDeployment(ctx context.Context, node *torchainv1alpha1.TorChain) error {
	var (
		interfaceName string
		gw            string
	)
	// 2.1
	// ---------------------------------- инициализация входного интерфейса ----------------------------------
	inNetConf, err := r.getNetworkDefinition(ctx, node.Namespace, node.Spec.InInterface)
	if err != nil {
		return err
	}
	inNetParams := NetDefinitionParams{}
	if len(inNetConf.Plugins) != 0 { // if init bridge adapter
		inNetParams = inNetConf.Plugins[0]
		interfaceName = inNetConf.Bridge
	} else { // else init macvlan adapter
		interfaceName = inNetConf.Master
	}
	inDeployAnnotation := &NetDeploymentAnnotation{
		Name:      node.Spec.InInterface,
		Interface: interfaceName,
	}
	// если входной узел цепочки
	if node.Spec.NumberNode == node.Spec.DropVPNChain {
		inDeployAnnotation.Ips = append(inDeployAnnotation.Ips, node.Spec.IPGateWay)
	} else {
		ipNode := net.ParseIP(inNetParams.Ipam.RangeStart)
		ipNode[len(ipNode)-1] += 2
		inDeployAnnotation.Ips = append(inDeployAnnotation.Ips, ipNode.String())
	}
	jsonInNetAnnotation, err := json.Marshal(inDeployAnnotation)
	if err != nil {
		return err
	}
	// ---------------------------------- инициализация выходного интерфейса ----------------------------------
	outNetConf, err := r.getNetworkDefinition(ctx, node.Namespace, node.Spec.OutInterface)
	if err != nil {
		return err
	}
	outNetParams := NetDefinitionParams{}
	if len(outNetConf.Plugins) != 0 { // if init bridge adapter
		outNetParams = outNetConf.Plugins[0]
		interfaceName = outNetConf.Bridge
	} else { // else init macvlan adapter
		interfaceName = outNetConf.Master
	}
	outDeployAnnotation := &NetDeploymentAnnotation{
		Name:      node.Spec.InInterface,
		Interface: interfaceName,
	}
	ipNode := net.ParseIP(outNetParams.Ipam.RangeStart)
	ipNode[len(ipNode)-1] += 1
	outDeployAnnotation.Ips = append(outDeployAnnotation.Ips, ipNode.String())
	// если выходной узел цепочки
	if node.Spec.NumberNode == 1 {
		gw = outNetParams.Ipam.GateWay
	} else {
		ipNode[len(ipNode)-1] += 1
		gw = ipNode.String()
	}
	jsonOutNetAnnotation, err := json.Marshal(outDeployAnnotation)
	if err != nil {
		return err
	}
	// 2.3 (create  deployment with sidecar)
	// 2.3.1 create secret
	secret, err := createSecret(ctx, node)
	if err != nil {
		return err
	}
	err = r.Create(ctx, secret)
	if err != nil {
		return err
	}
	// 2.3.2 create deployment
	var replicas int32 = 1
	dpl := &appsv1.Deployment{
		ObjectMeta: ctrl.ObjectMeta{
			Namespace: node.Namespace,
			Name:      node.Name,
			Labels:    map[string]string{"instance": "torgateway"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &v1.LabelSelector{
				MatchLabels: map[string]string{"instance": "torgateway"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: ctrl.ObjectMeta{
					Annotations: map[string]string{netattachdef.NetworkAttachmentAnnot: "[" + string(jsonInNetAnnotation) + "," + string(jsonOutNetAnnotation) + "]"},
					Labels:      map[string]string{"instance": "torgateway"},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: node.Name + "-sec",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: node.Name,
								},
							},
						},
						{
							Name: node.Name + "-tmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            node.Name,
							Image:           node.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Resources:       corev1.ResourceRequirements{},
							Env: []corev1.EnvVar{
								{
									Name:  "GW",
									Value: gw,
								},
								{
									Name:  "CONF",
									Value: "/config/client.vpn",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      node.Name + "-sec",
									MountPath: "/config",
								},
								{
									Name:      node.Name + "-tmp",
									MountPath: "/tmp",
								},
							},
						},
						{
							Name:  "sidecar-healthcheck",
							Image: "busybox",
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"wget", "-T2", "-O-", "ya.ru"},
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
								TimeoutSeconds:      2,
								FailureThreshold:    2,
							},
						},
					},
					NodeSelector: map[string]string{corev1.LabelHostname: node.Spec.NameK8sNode},
				},
			},
		},
	}

	return r.Create(ctx, dpl)
}

func (r *TorChainReconciler) getNetworkDefinition(ctx context.Context, nameSpace, name string) (*NetDefinitionConfig, error) {
	interfaceDef := &netattachdef.NetworkAttachmentDefinition{}
	err := r.Get(ctx, types.NamespacedName{Namespace: nameSpace, Name: name}, interfaceDef)
	if err != nil {
		if errors.IsNotFound(err) {
			// 2.2
			return nil, err // не были заданы интерфейсы, цепрочка не может быть построена
		}
		return nil, err // ошибка получения ресурса
	}
	netConfig := &NetDefinitionConfig{}
	err = json.Unmarshal([]byte(interfaceDef.Spec.Config), netConfig)
	if err != nil {
		return nil, err
	}

	return netConfig, nil
}

func createSecret(ctx context.Context, node *torchainv1alpha1.TorChain) (*corev1.Secret, error) {
	vpnConfig := make(map[string]string)
	vpnConfig[node.Spec.VpnFileConfig] = "get secret from Vault"
	// get secret from vault
	newSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: node.Namespace,
			Name:      node.Name,
		},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{},
		StringData: vpnConfig,
	}

	return newSecret, nil
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
