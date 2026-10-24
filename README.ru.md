# kube-dnr

[English](README.md) | **Русский**

Kubernetes Disaster-and-Recovery - консольная утилита для бэкапа и восстановления ресурсов Kubernetes кластера.

## Возможности

- **Бэкап всех ресурсов кластера** с возможностью выбора конкретных неймспейсов
- **Очистка манифестов** от метаданных статуса, системных аннотаций и других полей
- **Сохранение в ZIP архив** с организованной структурой по неймспейсам
- **Восстановление в другой кластер** с автоматическим созданием неймспейсов
- **Поддержка всех типов ресурсов** через Kubernetes Discovery API
- **Гибкая конфигурация** через JSON файл для исключения ресурсов, неймспейсов и подстрок

## Установка

```bash
go build -o kube-dnr
```

## Конфигурация

Конфигурация задается через JSON файл, который указывается с помощью флага `--config`. Конфигурационный файл позволяет:

- Исключать определенные типы ресурсов из бэкапа
- Исключать определенные неймспейсы
- Исключать ресурсы по подстрокам в имени
- Настраивать параметры S3 для загрузки бэкапов

Пример конфигурационного файла `config/config.json`:

```json
{
  "kubernetes": {
    "excludedresources": [
      "Pod",
      "PodMetrics",
      "NodeMetrics",
      "Event",
      "ReplicaSet"
    ],
    "excludednamespaces": [
      "kube-system",
      "kube-public",
      "kube-node-lease"
    ],
    "excludedsubstrings": [
      "sh.helm.release",
      "kube-root-ca.crt"
    ]
  },
  "s3": {
    "bucket": "my-backup-bucket",
    "endpoint": "https://s3.example.com",
    "region": "us-east-1",
    "accesskeyid": "YOUR_ACCESS_KEY",
    "secretaccesskey": "YOUR_SECRET_KEY",
    "usessl": true
  }
}
```

### Поля конфигурации

#### kubernetes
- `excludedresources` - список типов ресурсов (Kind), которые будут исключены из бэкапа
- `excludednamespaces` - список неймспейсов, которые будут пропущены при бэкапе
- `excludedsubstrings` - список подстрок; ресурсы, содержащие эти подстроки в имени, будут пропущены

#### s3
- `bucket` - имя S3 бакета для хранения бэкапов
- `endpoint` - URL endpoint S3 совместимого хранилища
- `region` - регион S3
- `accesskeyid` - ключ доступа
- `secretaccesskey` - секретный ключ
- `usessl` - использовать SSL для подключения

## Использование

### Режим бэкапа

Создание бэкапа всех неймспейсов кластера:

```bash
./kube-dnr --config config/config.json backup -A -o backup.zip
```

Создание бэкапа конкретных неймспейсов:

```bash
./kube-dnr --config config/config.json backup -n default,kube-system -o backup.zip
```

Создание бэкапа с указанием кастомного kubeconfig:

```bash
./kube-dnr --config config/config.json backup -A -o backup.zip --kubeconfig /path/to/kubeconfig
```

#### Флаги команды backup

- `-A, --all-namespaces` - Бэкапить все неймспейсы кластера
- `-n, --namespaces` - Список неймспейсов для бэкапа (через запятую)
- `-o, --output` - Путь к выходному архиву (обязательно)
- `--kubeconfig` - Путь к kubeconfig файлу (по умолчанию ~/.kube/config)
- `--upload-s3` - Загрузить бэкап в S3 после создания (требует настройки S3 в конфиге)

#### Глобальные флаги

- `--config` - Путь к конфигурационному файлу (обязательно)

### Режим восстановления

Восстановление из архива в текущий кластер (определяется из KUBECONFIG):

```bash
./kube-dnr --config config/config.json restore -i backup.zip
```

Восстановление с указанием другого kubeconfig:

```bash
export KUBECONFIG=/path/to/other-cluster/config
./kube-dnr --config config/config.json restore -i backup.zip
```

Или:

```bash
./kube-dnr --config config/config.json restore -i backup.zip --kubeconfig /path/to/other-cluster/config
```

#### Флаги команды restore

- `-i, --input` - Путь к архиву бэкапа (обязательно, если не используется --download-s3)
- `--kubeconfig` - Путь к kubeconfig файлу (по умолчанию ~/.kube/config)
- `--download-s3` - Скачать бэкап из S3 (требует настройки S3 в конфиге)
- `--s3-object` - Имя объекта в S3 (если не указано, используется последний объект)

#### Глобальные флаги

- `--config` - Путь к конфигурационному файлу (обязательно)

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
./kube-dnr --config config/config.json backup -A -o full-backup.zip

# Восстановление в другой кластер
export KUBECONFIG=/path/to/new-cluster/config
./kube-dnr --config config/config.json restore -i full-backup.zip
```

### Бэкап конкретного приложения

```bash
# Бэкап только неймспейса с приложением
./kube-dnr --config config/config.json backup -n my-app -o my-app-backup.zip

# Восстановление в другом кластере
./kube-dnr --config config/config.json restore -i my-app-backup.zip --kubeconfig /path/to/new-cluster/config
```

### Бэкап с загрузкой в S3

```bash
# Бэкап всех ресурсов и загрузка в S3
./kube-dnr --config config/config.json backup -A -o backup.zip --upload-s3
```

### Восстановление из S3

```bash
# Восстановление из последнего бэкапа в S3
./kube-dnr --config config/config.json restore --download-s3

# Восстановление из конкретного объекта в S3
./kube-dnr --config config/config.json restore --download-s3 --s3-object 20260121-150405/backup.zip
```

## Требования

- Go 1.27.1 или выше
- Доступ к Kubernetes кластеру через kubeconfig
- Необходимые права RBAC для:
  - Бэкап: `list` на всех ресурсах
  - Восстановление: `create`, `update`, `get` на всех ресурсах

## Лицензия

См. файл LICENSE
