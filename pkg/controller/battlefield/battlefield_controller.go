package battlefield

import (
	"context"
	"strconv"
	"strings"

	rhtev1alpha1 "github.com/bszeti/battlefield-operator/pkg/apis/rhte/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_battlefield")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Battlefield Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileBattlefield{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("battlefield-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Battlefield
	err = c.Watch(&source.Kind{Type: &rhtev1alpha1.Battlefield{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Battlefield
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &rhtev1alpha1.Battlefield{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileBattlefield implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileBattlefield{}

// ReconcileBattlefield reconciles a Battlefield object
type ReconcileBattlefield struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Battlefield object and makes changes based on the state read
// and what is in the Battlefield.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileBattlefield) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Battlefield")

	// Fetch the Battlefield instance
	battlefield := &rhtev1alpha1.Battlefield{}
	err := r.client.Get(context.TODO(), request.NamespacedName, battlefield)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	reqLogger.Info("Battlefield", "Name", battlefield.ObjectMeta.Name,  "Duration", battlefield.Spec.Duration, "HitFrequency", battlefield.Spec.HitFrequency, "Num of players", len(battlefield.Spec.Players) )

	for _, player := range battlefield.Spec.Players {
		// Define a new Pod object
		pod := newPodForCR(battlefield, &player)
		
		// Set Battlefield instance as the owner and controller
		if err := controllerutil.SetControllerReference(battlefield, pod, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		// Check if this Pod already exists
		found := &corev1.Pod{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			err = r.client.Create(context.TODO(), pod)
			if err != nil {
				return reconcile.Result{}, err
			}

		} else if err != nil {
			return reconcile.Result{}, err
		}

		// Pod already exists - don't requeue
		reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
		
	}

	return reconcile.Result{}, nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(battlefield *rhtev1alpha1.Battlefield, player *rhtev1alpha1.Player) *corev1.Pod {
	labels := map[string]string{
		"app": battlefield.Name,
		"battlefield": battlefield.Name,
	}

	var targets []string
	for _, target := range battlefield.Spec.Players {
		if player.Name != target.Name { 
			targets = append(targets,battlefield.Name + "-player-" + target.Name)
		}
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      battlefield.Name + "-player-" + player.Name,
			Namespace: battlefield.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:	"player",
					Image:	player.Image,
					ImagePullPolicy: 	corev1.PullAlways,
					Env: []corev1.EnvVar{
						{
							Name:  "BATTLEFIELD_PLAYER_NAME",
							Value: player.Name,
						},
						{
							Name:  "BATTLEFIELD_PLAYER_URLS",
							Value: strings.Join(targets,","),
						},
						{
							Name:  "BATTLEFIELD_MAX_HEALTH",
							Value: strconv.Itoa(player.MaxHealth),
						},
						{
							Name:  "BATTLEFIELD_HIT_PERIOD",
							Value: strconv.Itoa(battlefield.Spec.HitFrequency * 1000),
						},
					},
				},
			},
			RestartPolicy: 		corev1.RestartPolicyNever,
		},
	}
}
