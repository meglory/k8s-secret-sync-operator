package secret

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
)

var log = logf.Log.WithName("controller_secret")

func RegFunc(mgr manager.Manager) error {
	return reg(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSecret{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

func reg(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New("secretsync-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &ReconcileSecret{}

type ReconcileSecret struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Secret")

	instance := &corev1.Secret{}
	err := r.client.Get(context.Background(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if targetNamespaces, ok := instance.Annotations["secretsync.ndp.netease.com/to-namespaces"]; ok {
		reqLogger.Info(fmt.Sprintf("secret [%s] in [%s] namespace is configured sync to [%s].", instance.Name, instance.Namespace, targetNamespaces))
		namespaces := make([]string, 0)
		if targetNamespaces != "" {
			namespaces = strings.Split(targetNamespaces, ",")
		}
		// 先执行删除逻辑
		if err := handleDelete(r, instance, namespaces, reqLogger); err != nil {
			return reconcile.Result{}, err
		}
		// 再执行复制逻辑
		if err := handleReplica(r, instance, namespaces, reqLogger); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// 处理清除逻辑
func handleDelete(r *ReconcileSecret, instance *corev1.Secret, targetNamespaces []string, reqLogger logr.Logger) error {
	// 根据from-name、from-namespace label查找所有命名空间的secret，如果对应命名空间已删除，则删除该secret
	secrets := &corev1.SecretList{}
	err := r.client.List(context.Background(), client.MatchingLabels(map[string]string{
		"secretsync.ndp.netease.com/from-name":      fmt.Sprintf("%s", instance.Name),
		"secretsync.ndp.netease.com/from-namespace": fmt.Sprintf("%s", instance.Namespace),
	}), secrets)
	if err != nil {
		return err
	}
	for _, secret := range secrets.Items {
		if !contains(targetNamespaces, secret.Namespace) {
			reqLogger.Info(fmt.Sprintf("secret [%s] in [%s] namespace needs to cleanup.", secret.Name, secret.Namespace))
			err = r.client.Delete(context.Background(), &secret)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// 处理同步逻辑
func handleReplica(r *ReconcileSecret, instance *corev1.Secret, targetNamespaces []string, reqLogger logr.Logger) error {
	for _, ns := range targetNamespaces {
		replicaSecret, err := createReplicaSecret(instance, ns)
		if err != nil {
			return err
		}
		secret := &corev1.Secret{}
		err = r.client.Get(context.Background(), types.NamespacedName{Name: replicaSecret.Name, Namespace: replicaSecret.Namespace}, secret)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info(fmt.Sprintf("secret [%s] doesn't exists in [%s] namespace, creating it.", replicaSecret.Name, replicaSecret.Namespace))
			err = r.client.Create(context.Background(), replicaSecret)
			if err != nil {
				return err
			}
		} else {
			reqLogger.Info(fmt.Sprintf("secret [%s] already exists in [%s] namespace, updating it now.", replicaSecret.Name, replicaSecret.Namespace))
			err = r.client.Update(context.Background(), replicaSecret)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func createReplicaSecret(secret *corev1.Secret, namespace string) (*corev1.Secret, error) {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: secret.TypeMeta.APIVersion,
			Kind:       secret.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: namespace,
			Labels:    patchLabels(secret),
		},
		Data: secret.Data,
		Type: secret.Type,
	}, nil
}

func contains(arr []string, item string) bool {
	if len(arr) == 0 || item == "" {
		return false
	}
	for _, str := range arr {
		if str == item {
			return true
		}
	}
	return false
}

func patchLabels(secret *corev1.Secret) map[string]string {
	res := map[string]string{}
	for k, v := range secret.Labels {
		res[k] = v
	}
	res["secretsync.ndp.netease.com/from-name"] = fmt.Sprintf("%s", secret.Name)
	res["secretsync.ndp.netease.com/from-namespace"] = fmt.Sprintf("%s", secret.Namespace)
	res["secretsync.ndp.netease.com/from-uuid"] = fmt.Sprintf("%s", secret.UID)
	return res
}
