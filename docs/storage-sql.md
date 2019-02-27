# SQL storage

If you have very large charts (above 1MB), using SQL storage is a suggested solution. Etcd has a file size limit of 1MB which will cause deployments of very large charts to fail.

## Usage

You need to start tiller up to use the disk storage option

```shell
helm init --override 'spec.template.spec.containers[0].command'='{/tiller,--storage=sql}'
```

If you want to have a versioned manifest of your tiller, here is one you can reuse:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: tiller-fork-cluster-rule
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: tiller-fork
    namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cluster-admin-fork
rules:
  - apiGroups:
      - "*"
    resources:
      - "*"
    verbs:
      - "*"
  - nonResourceURLs:
      - "*"
    verbs:
      - "*"
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tiller-fork
  namespace: kube-system
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: helm
    name: tiller-fork
  name: tiller-fork-deploy
  namespace: kube-system
spec:
  clusterIP: 100.65.44.199
  ports:
    - name: tiller-fork
      port: 44134
      protocol: TCP
      targetPort: tiller-fork
  selector:
    app: helm
    name: tiller-fork
  sessionAffinity: None
  type: ClusterIP
status:
  loadBalancer: {}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: helm
    name: tiller-fork
  name: tiller-fork-deploy
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: helm
      name: tiller-fork
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: helm
        name: tiller-fork
    spec:
      automountServiceAccountToken: true
      containers:
        - command:
          - /tiller
          - --storage=sql
        - env:
          - name: TILLER_NAMESPACE
            value: kube-system
          - name: TILLER_HISTORY_MAX
            value: "0"
          image: gcr.io/kubernetes-helm/tiller:v2.12.3
          imagePullPolicy: IfNotPresent
          livenessProbe:
            failureThreshold: 3
            httpGet:
              path: /liveness
              port: 44135
              scheme: HTTP
            initialDelaySeconds: 1
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 1
          name: tiller-fork
          ports:
            - containerPort: 44134
              name: tiller-fork
              protocol: TCP
            - containerPort: 44135
              name: http
              protocol: TCP
          readinessProbe:
            failureThreshold: 3
            httpGet:
              path: /readiness
              port: 44135
              scheme: HTTP
            initialDelaySeconds: 1
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 1
          resources: {}
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      serviceAccount: tiller-fork
      serviceAccountName: tiller-fork
      terminationGracePeriodSeconds: 30
```

You will need a spin up a PostgresQL to save the releases states. Here is a manifst you can reuse in order to do so:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-tiller-config
  labels:
    app: postgres-tiller
data:
  POSTGRES_DB: postgresdb
  POSTGRES_USER: postgresadmin
  POSTGRES_PASSWORD: admin123
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-tiller-init
  labels:
    app: postgres-tiller
data:
  init.sql: |
    CREATE DATABASE tiller;
    GRANT ALL PRIVILEGES ON DATABASE tiller TO postgresadmin;
---
apiVersion: v1
kind: Service
metadata:
  name: postgres-tiller
spec:
  ports:
    - port: 5432
  selector:
    app: postgres-tiller
---
kind: PersistentVolume
apiVersion: v1
metadata:
  name: postgres-tiller-pv-volume
  labels:
    type: local
    app: postgres-tiller
spec:
  storageClassName: manual
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteMany
  hostPath:
    path: "/mnt/data"
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: postgres-tiller-pv-claim
  labels:
    app: postgres-tiller
spec:
  storageClassName: manual
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres-tiller
spec:
  serviceName: postgres-tiller
  selector:
    matchLabels:
      app: postgres-tiller
  replicas: 1
  template:
    metadata:
      labels:
        app: postgres-tiller
    spec:
      containers:
        - name: postgres-tiller
          image: postgres:11.2
          imagePullPolicy: Always
          ports:
            - containerPort: 5432
          envFrom:
            - configMapRef:
                name: postgres-tiller-config
          resources:
            limits:
              memory: 128Mi
            requests:
              cpu: 50m
              memory: 128Mi
          volumeMounts:
            - mountPath: /var/lib/postgresql/data
              name: postgredb
            - mountPath: /docker-entrypoint-initdb.d
              name: postgres-init
      volumes:
        - name: postgredb
          persistentVolumeClaim:
            claimName: postgres-tiller-pv-claim
        - name: postgres-init
          configMap:
            name: postgres-tiller-init
      nodeSelector:
        duty: storage
```