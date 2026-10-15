# kube-dnr

Kubernetes Disaster-and-Recovery - консольная утилита для бэкапа и восстановления ресурсов Kubernetes кластера.

## Возможности

- **Бэкап всех ресурсов кластера** с возможностью выбора конкретных неймспейсов
- **Очистка манифестов** от метаданных статуса, системных аннотаций и других полей
- **Сохранение в ZIP архив** с организованной структурой по неймспейсам
- **Восстановление в другой кластер** с автоматическим созданием неймспейсов
- **Поддержка всех типов ресурсов** через Kubernetes Discovery API

## Установка

```bash
go build -o kube-dnr
```

## Использование

### Режим бэкапа

Создание бэкапа всех неймспейсов кластера:

```bash
./kube-dnr backup -A -o backup.zip
```

Создание бэкапа конкретных неймспейсов:

```bash
./kube-dnr backup -n default,kube-system -o backup.zip
```

Создание бэкапа с указанием кастомного kubeconfig:

```bash
./kube-dnr backup -A -o backup.zip --kubeconfig /path/to/kubeconfig
```

#### Флаги команды backup

- `-A, --all-namespaces` - Бэкапить все неймспейсы кластера
- `-n, --namespaces` - Список неймспейсов для бэкапа (через запятую)
- `-o, --output` - Путь к выходному архиву (обязательно)
- `--kubeconfig` - Путь к kubeconfig файлу (по умолчанию ~/.kube/config)

### Режим восстановления

Восстановление из архива в текущий кластер (определяется из KUBECONFIG):

```bash
./kube-dnr restore -i backup.zip
```

Восстановление с указанием другого kubeconfig:

```bash
export KUBECONFIG=/path/to/other-cluster/config
./kube-dnr restore -i backup.zip
```

Или:

```bash
./kube-dnr restore -i backup.zip --kubeconfig /path/to/other-cluster/config
```

#### Флаги команды restore

- `-i, --input` - Путь к архиву бэкапа (обязательно)
- `--kubeconfig` - Путь к kubeconfig файлу (по умолчанию ~/.kube/config)

## Структура архива

Архив бэкапа организован следующим образом:

```
backup.zip
├── namespaces/
│   ├── default/
│   │   ├── pods/
│   │   │   └── my-pod.yaml
│   │   ├── services/
│   │   │   └── my-service.yaml
│   │   └── deployments/
│   │       └── my-deployment.yaml
│   └── kube-system/
│       └── ...
└── cluster-wide/
    ├── nodes/
    │   └── node1.yaml
    ├── persistentvolumes/
    │   └── pv1.yaml
    └── namespaces/
        └── default.yaml
```

## Что очищается из манифестов

При бэкапе из ресурсов удаляются следующие поля для обеспечения переносимости:

- `status` - статус ресурса
- `metadata.uid` - уникальный идентификатор
- `metadata.selfLink` - ссылка на ресурс
- `metadata.creationTimestamp` - время создания
- `metadata.generation` - поколение ресурса
- `metadata.resourceVersion` - версия ресурса
- `metadata.managedFields` - поля управляемые контроллерами
- `metadata.ownerReferences` - поля управляемые другими приложениями или операторами
- Системные аннотации (kubectl.kubernetes.io/last-applied-configuration и др.)
- Системные лейблы (pod-template-hash, controller-uid и др.)
- Специфичные поля (например, secrets в ServiceAccount)

## Примеры

### Полный бэкап и восстановление

```bash
# Бэкап всех ресурсов кластера
./kube-dnr backup -A -o full-backup.zip

# Восстановление в другой кластер
export KUBECONFIG=/path/to/new-cluster/config
./kube-dnr restore -i full-backup.zip
```

### Бэкап конкретного приложения

```bash
# Бэкап только неймспейса с приложением
./kube-dnr backup -n my-app -o my-app-backup.zip

# Восстановление в другом кластере
./kube-dnr restore -i my-app-backup.zip --kubeconfig /path/to/new-cluster/config
```

## Требования

- Go 1.27.1 или выше
- Доступ к Kubernetes кластеру через kubeconfig
- Необходимые права RBAC для:
  - Бэкап: `list` на всех ресурсах
  - Восстановление: `create`, `update`, `get` на всех ресурсах

## Лицензия

См. файл LICENSE
