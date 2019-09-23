package controller

import (
	"github.com/meglory/k8s-secret-sync-operator/pkg/controller/secret"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// RegisterFuncs controller注册函数
var RegisterFuncs []func(manager.Manager) error
var SchemeBuilder runtime.SchemeBuilder

func init() {
	RegisterFuncs = append(RegisterFuncs, secret.RegFunc)
}

// RegisterToManager执行自定义controller注册函数
func RegisterToManager(m manager.Manager) error {
	for _, f := range RegisterFuncs {
		if err := f(m); err != nil {
			return err
		}
	}
	return nil
}

// 添加scheme
func AddScheme(s *runtime.Scheme) error {
	return SchemeBuilder.AddToScheme(s)
}
