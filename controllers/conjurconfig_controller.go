/*
Copyright 2021.

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

	"github.com/go-logr/logr"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsk8siov1 "github.com/diverdane/conjur-config-operator/api/v1"
)

// ConjurConfigReconciler reconciles a ConjurConfig object
type ConjurConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apiextensions.k8s.io.cyberark.com,resources=conjurconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apiextensions.k8s.io.cyberark.com,resources=conjurconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apiextensions.k8s.io.cyberark.com,resources=conjurconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ConjurConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *ConjurConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("conjurconfig", req.NamespacedName)

	// Fetch the ConjurConfig instance
	conjurConfig := &apiextensionsk8siov1.ConjurConfig{}
	err := r.Get(ctx, req.NamespacedName, conjurConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("ConjurConfig resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get ConjurConfig")
		return ctrl.Result{}, err
	}

	// Check if the ConfigMap already exists, if not create a new one
	found := &v1.ConfigMap{}
	cmName := conjurConfig.Spec.ConfigMapName
	cmNamespace := conjurConfig.Namespace
	err = r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: cmNamespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new ConfigMap
		cm := r.configMapForConjurConfig(conjurConfig)
		log.Info("Creating a new ConfigMap", "ConfigMap.Name", cmName,
			"ConfigMap.Namespace", cmNamespace)
		err = r.Create(ctx, cm)
		if err != nil {
			log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Name", cm.Name, "ConfigMap.Namespace", cm.Namespace)
			return ctrl.Result{}, err
		}
		// ConfigMap created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	// TODO: Ensure ConfigMap has correct content

	// TODO: Add ConfigMap created and/or timestamp to status?

	return ctrl.Result{}, nil
}

// configMapForConjurConfig returns a Conjur connect ConfigMap object
func (r *ConjurConfigReconciler) configMapForConjurConfig(
	c *apiextensionsk8siov1.ConjurConfig) *v1.ConfigMap {

	ls := labelsForConjurConfig(c.Name)

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Spec.ConfigMapName,
			Namespace: c.Namespace,
			Labels:    ls,
		},
		Data: map[string]string{
			"CONJUR_APPLIANCE_URL": "https://conjur-oss.conjur-oss.svc.cluster.local",
		},
	}
	// Set ConjurConfig instance as the owner and controller
	ctrl.SetControllerReference(c, configMap, r.Scheme)
	return configMap
}

// labelsForConjurConfig returns the labels for selecting the resources
// belonging to the given ConjurConfig CR name.
func labelsForConjurConfig(name string) map[string]string {
	return map[string]string{"app": "conjur-config", "conjur-config-cr": name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConjurConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiextensionsk8siov1.ConjurConfig{}).
		Owns(&v1.ConfigMap{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
