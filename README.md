# VPN шлюз с множественной инкапсуляцией (аналог TOR-сети)

Цель данного решения - продемонстрировать нестандартное использование системы управления контейнерами на базе Kubernetes, 
так как k8s - это SDK :)

# Начальные условия 

Для реализации луковичной маршрутизации (построение VPN цепочки с произвольным количеством узлов) необходимо использовать CNI Multus (для задания маршрутов на L2 уровне) + базовый плагин с поддержкой NP (для сетевой изоляции ПОДов в сети CIDR K8s на уровне NS), например Calico

Предполагается использовать протоколы OnenVPN и Wireguard в любом сочетании.

# Функциональность

Развертывание производится через helm chart (папка Helm/tor-gateway).
В templates задаются сетевые интерфейсы через NetworkAttachmentDefinition и параметры узлов цепочки через CRD разработанного kube-оператора. Параметры подключения хранятся в secrets (Vault) и монтируются как файл в файловой системе ПОДа.

Если какой-либо из узлов цепочки не может подключиться к серверу, то оператор берет из Vault новые параметры подключения и перезапускает узел.

Так как для контейнеров необходимы привилегии NET_ADMIN, docker-образы собраны через nixpkgs docker build (папки Docker/ovpn-build-image и Docker/wguard-build-image) с минимальным окружением. Данный подход позволяет минимизировать вектор атаки через VPN-клиента. 
Использование distroless-образов приводит к необходимости запускать kube-оператором sidecar-контейнеры (пр.: busybox) c health check для отслеживания состояния работы ПОДа (узла цепочки).

VPN-клиенты логируют сообщения в stdout.

# Этапы работ

На первом этапе проводились тестовые испытания: была развернута цепочка client1-client2-client3-server3-server2-server1 через docker-compose (в проекте не представлено).

На втором была поднята K8s через kubeadm (папка K8s). Облачные провайдеры не подходят из-за использования CNI Multus и необходимости знания топологии физической сети кластера.

На третем - разработан helm chart (папка Helm/ovpn-route) развертывания цепочки, состоящей из 3-х OpenVPN узлов.

На четвертом этапе разработан оператор, который создает узлы цепочки в соответствии с заданной в chart-е (папка Helm/tor-gateway) концигурацией и отслеживает состояние доступности внешних узлов.

На текущем этапе - интеграция с ELK.

# HELM Chart

Необходимость Chart-а обусловлена описанием концигурации сетевых интерфейсов клиентских узлов цепочки в соответствии с конфигурацией физической сети кластера, а также заданием параметров каждого узла цепочки на основе CRD TorChain:

    type TorChainSpec struct {
		//drop of vpn chain
		DropVPNChain int `json:"drop,omitempty"`
		// number node of chain
		NumberNode int `json:"numberNode,omitempty"` // 1 or 2 or 3
		// environments:
		// ip gateway
		IPGateWay string `json:"ipGateWay,omitempty"`
		// list of VPN configs in vault
		VpnSecretNames []string `json:"vpnSecretNames,omitempty"`
		// current VPN config
		VpnSecretName string `json:"vpnSecretName,omitempty"`
		// interfaces:
		// input traffic
		InInterface string `json:"inInterface,omitempty"`
		// output traffic
		OutInterface string `json:"outInterface,omitempty"`
		// image VPN client
		Image string `json:"image,omitempty"`
		// nodeSelector
		NameK8sNode string `json:"nameK8sNode,omitempty"`
	}

В примере описывается (по мнению автора) оптимальная конфигурация:
1. Все узлы цепочки запускаются на определенной НОДе кластера (через NodeSelector)
2. Входной интерфейс цепочки с фиксированным IP-GateWay создается как macvlan. Macvlan позволяет создавать виртуальный интерфейс, наследованный от родительского физического интерфейса, с уникальным MAC-адресом. Что дает входящему узлу цепочки возможность подключаться к физической сети кластера.
3. Промежуточные интерфейсы создаются как мосты от local-интерфеса НОДы
4. Выходной интерфейс - мост от физического интерфейса НОДы внешней сети кластера.

# Алгоритм работы оператора

Целью разработки оператора было поддержание в автоматизированном режиме рабочей VPN-цепочки при наличии пула внешних VPN-серверов c конфигурационными файлами доступа в (защищенном) хранилище (пр. Vault). Таким образом, при недоступности внешнего сервера какого-либо узла цепочки, оператор должен взять из хранилища конфигурационные данные доступа к новому серверу, перезаписать данные и перезапустить узел. 
Для решения задачи генарации события update (или patch) основной функции Reconcile оператора при недоступности сервера узла цепочки было найдено два способа:
1. Направлять логи ошибок VPN-клиентов в Prometheus, строить метрики (например, количество ошибок подключения к VPN-серверу) и через prometheus-adapter изменять значение в спецификации манифеста узла цепочки в базе etcd.
2. В операторе создать отдельный поток, который через заданный интервал времени получает состояние узлов цепочек и при изменении состояния изменяет значение в спецификации манифеста узла цепочки в базе etcd.  

Был выбран второй способ. 

Алгоритм:
1. При первом запуске Reconcile создается горутина сканирования узлов всех цепочек (функция snifferTorChains)
2. При создании CRD TorChain оператор создает secret и Deployment (функция createDeployment), в котором задан базовый контейнер VPN-клиента и sidecar-контейнер с livenessProbe проверки доступности VPN-сервера.
3. Функция snifferTorChains:
    - Определяет в каком NameSpace есть ПОД со статусом false.
    - Так как 'вехний' по вложенности ПОД, в случае недоступности сервера, будет блокировать трафик всем ПОДам уровней 'ниже', в найденном NameSpace функция находит самый 'верхний' (наименьший по нумерации) ПОД со статусом false. 
     - Для найденного ПОДа изменяется значение VpnSecretName в спецификации и обновляется манифест (для инициализации события update). В значении указывается имя следующего секрета (параметры доступа к следующему серверу) из VpnSecretNames, который необходимо применить. 
   
При такой реализации будет происходить последовательное восстановление связи от 'верхнего' узла цепочки к 'нижнему' (по вложенности). 
   
Таймаут итерации сканирования в snifferTorChains задан больше, чем PeriodSeconds+TimeoutSeconds в LivenessProbe. Это условие позволяет гарантировать, что snifferTorChains не обновит спецификацию манифеста прежде, чем не отработает LivenessProbe после предыдущего обновления.

# Vault

Логика оператора основана на интеграции с Vault через vault-secrets-operator. 

Перед инициализацией VPN-цепочки необходимо:
 - На сервисе Vault:
   1. Инициализировать протокол и метод аутентификации в vault - kubernetes
   2. Инициировать секрет типа ключ-значение по пути kvv2
   3. Создать роль на доступ к секретам (read) и привязать ее к сервис-акканту пользователя, который будет создавать цепочку, а так же привязать к namespace, в котором будет развернута цепочка (Release.Name в helm chart!).
   4. Записать необходимое количество параметров доступа к серверам. Например:
		- vault kv put kvv2/ns/number-node-chain/domain/city/server/wg client.vpn="creds-in-base64"
		- vault kv put kvv2/ns/number-node-chain/domain/city/server/ovpn client.vpn="creds-in-base64"

			где:
			- ns - namespace (Release.Name)
			- number-node-chain - номер узла в цепочке 
			- domain - страна
			- city - город (регион)
			- server - сервер
			- ovpn/wp - тип подключения

	Ключ строго фиксирован (client.vpn), так как по имени ключча создается конфигурационный файл. Данное имя задано в коде оператора.
 - В кластере:
	1. Инициировать аутентификацию для секрета:
		Например:
        ```
			apiVersion: secrets.hashicorp.com/v1beta1
			kind: VaultAuth
			metadata:
  			  name: static-auth
  			  namespace: {{ $.Release.Name }}	
			spec:
  			  method: kubernetes
  			  mount: tor-auth-mount # auth path
  			kubernetes:
    		  role: role-name 
    		  serviceAccount: user-service-account # default
    		  audiences:
      		    - vault
        ```
	2. Создать ресурсы VaultStaticSecret, которые монтирую конфигурационные параметры доступа (п.4) к label (имени секрета)
		Например:
        ```
			apiVersion: secrets.hashicorp.com/v1beta1
			kind: VaultStaticSecret
			metadata:
			  name: domain1-city1-server1
			  namespace: {{ $.Release.Name }}	
			spec:
			  type: kv-v2
			  # mount path
			  mount: kvv2
			  # path of the secret
			  path: ns/number-node-chain/domain/city/server/wg
			  # dest k8s secret
			  destination:
				name: domain1-city1-server1
				create: true
			  # static secret refresh interval
			  refreshAfter: 1h
			  # Name of the CRD to authenticate to Vault
			  vaultAuthRef: static-auth
        ```
Таким образом, в переменной кастомного ресурса tor-оператора VpnSecretNames задается список имен секретов (из ресурсов VaultStaticSecret), которые можно задавать для конкретного узла цепочки. 

# Сборка образа при помощи пакетного менеджера nix (nixpkgs)

nixpkgs - пакетный менеджер, построенный вокруг идеи декларативного описания конфигурации (или состояния) приложения или системы (NixOS).

Nix размещает все установленные пакеты в собственных подкаталогах внутри каталога /nix/store. К примеру, установленный пакет Git будет располагаться в каталоге /nix/store/nawl092prjblbhvv16kxxbk6j9gkgcqm-git-2.14.1, где набор цифр — это хеш, образованный от окружения сборки пакета: файлов исходников, дерева зависимостей, флагов компилятора и другого. Поэтому с помощью Nix можно установить одновременно не только две версии одного приложения, но и даже две разные сборки.

При указании в декларации описания конфигурации пакета docker-образ (pkgs.dockerTools.buildImage {}), в результате сборки (nix-build default.nix) получится архив, скомпанованный в формате OCI (см. https://nix.dev/tutorials/nixos/building-and-running-docker-images.html).


# tor-operator
// TODO(user): Add simple overview of use/purpose

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/tor-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/tor-operator:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

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

## Command steps

operator-sdk init --domain gate.way --owner "Lixoi" --plugins go/v3
operator-sdk create api --group torchain --version v1alpha1 --kind TorChain --resource --controller
// edit api/v1alpha1/totchain_types.go
make manifests
operator-sdk create webhook --group torchain --version v1alpha1 --kind TorChain --defaulting --programmatic-validation