package battlefield

import (
	"context"
	"strconv"
	"strings"
	"time"

	rhtev1alpha1 "github.com/bszeti/battlefield-operator/pkg/apis/rhte/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	reqLogger := log.WithValues("Battlefield", request.Name)
	ctx := context.TODO()

	reqLogger.Info("Reconciling Battlefield")

	// Fetch the Battlefield instance
	battlefield := &rhtev1alpha1.Battlefield{}
	err := r.client.Get(ctx, request.NamespacedName, battlefield)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Error reading Battlefield")
		return reconcile.Result{}, err
	}

	timeExpired := false
	//***********************
	// Init Battlefield
	//***********************
	if battlefield.Status.Phase == "" {
		reqLogger.Info("Init game", "Name", battlefield.ObjectMeta.Name, "Duration", battlefield.Spec.Duration, "HitFrequency", battlefield.Spec.HitFrequency, "Num of players", len(battlefield.Spec.Players))

		battlefield.Status.Phase = "init"

		//Init player scores
		battlefield.Status.Scores = nil
		for _, player := range battlefield.Spec.Players {
			battlefield.Status.Scores = append(battlefield.Status.Scores,
				rhtev1alpha1.PlayerStatus{
					Name:  player.Name,
					Kill:  0,
					Death: 0,
				})
		}

		//Init times - if this is a restart
		battlefield.Status.StartTime = nil
		battlefield.Status.StopTime = nil

		err := r.client.Status().Update(ctx, battlefield)
		if err != nil {
			reqLogger.Error(err, "Error updating Battlefield")
		}
		//The update will trigger a new request. To avoid racing conditions, it's better to always return.
		return reconcile.Result{}, err

		
	} 
	//check if time is expired
	if battlefield.Status.StartTime != nil {
		timeExpired = time.Since(battlefield.Status.StartTime.Time) > time.Duration(battlefield.Spec.Duration)*time.Second
	}
	

	//reqLogger.Info("Players", "Num of players", len(battlefield.Spec.Players), "Names", battlefield.Spec.Players )

	//***********************
	// Manage services
	//***********************
	for _, player := range battlefield.Spec.Players {
		service := newServiceForPlayer(battlefield, &player)

		//reqLogger.Info("Service", "name", service.ObjectMeta.Name)

		// Set Battlefield instance as the owner and controller
		if err := controllerutil.SetControllerReference(battlefield, service, r.scheme); err != nil {
			reqLogger.Error(err, "Error setting owner for Service", "Service.Name", service.Name)
			//TODO: break instead
			return reconcile.Result{}, err
		}

		// Check if this Service already exists
		found := &corev1.Service{}
		err := r.client.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
		if err != nil {
			if errors.IsNotFound(err) {
				if !timeExpired { //Create missing service if time is not expired
					reqLogger.Info("Creating Service", "Service.Name", service.Name)
					err := r.client.Create(ctx, service)
					if err != nil {
						reqLogger.Error(err, "Error creating Service", "Service.Name", service.Name)
						return reconcile.Result{}, err
					}
				}
			} else {
				reqLogger.Error(err, "Error reading Service", "Service.Name", service.Name)
				return reconcile.Result{}, err
			}
		} else {
			//Service is found

			//If time is up, delete service
			if timeExpired && found.DeletionTimestamp == nil {
				reqLogger.Info("Time is up - deleting service", "Service.Name", found.Name)
				err := r.client.Delete(ctx, found)
				if err != nil {
					reqLogger.Error(err, "Error deleting Service", "Service.Name", found.Name)
					return reconcile.Result{}, err
				}
			}
		}
	}

	//***********************
	// Manage pods
	//***********************
	for _, player := range battlefield.Spec.Players {
		// Define a new Pod object
		pod := newPodForPlayer(battlefield, &player)
		if err := controllerutil.SetControllerReference(battlefield, pod, r.scheme); err != nil {
			reqLogger.Error(err, "Error setting owner for Pod", "Pod.Name", pod.Name)
			//TODO: break instead?
			return reconcile.Result{}, err
		}

		// Check if pod already exists
		found := &corev1.Pod{}
		err := r.client.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
		if err != nil {
			if errors.IsNotFound(err) {
				if !timeExpired {
					// Create missing pod - (re)spawn player
					reqLogger.Info("Creating Pod", "Pod.Name", pod.Name)
					err := r.client.Create(ctx, pod)
					if err != nil {
						reqLogger.Error(err, "Error creating Pod", "Pod.Name", pod.Name)
						return reconcile.Result{}, err
					}
				}
			} else {
				reqLogger.Error(err, "Error reading Pod", "Pod.Name", pod.Name)
				return reconcile.Result{}, err
			}
		} else {
			// Pod is found
			//reqLogger.Info("Pod status", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name, "Status", found.Status)
			
			//check if container "player" is terminated - update scores and delete the pod to stop any sidecars
			containerStatusPlayer := getContainerStatus(found.Status.ContainerStatuses, "player")
			if containerStatusPlayer != nil  { //It's expected to have a "player" container...
				setPlayerReady(battlefield,player.Name,containerStatusPlayer.Ready)

				//Only manage pod if it's not marked for deletetion yet
				if found.DeletionTimestamp == nil {
					if containerStatusPlayer.State.Terminated != nil {
						//TODO: Sometimes points are counted multiple times because of racing conditions. client.Delete doesn't return if Pod was deleted before.

						reqLogger.Info("Killed - deleting Pod", "Pod.Name", found.Name)
						err = r.client.Delete(ctx, found)
						if err != nil {
							reqLogger.Error(err, "Error deleting Pod", "Pod.Name", found.Name)
							return reconcile.Result{}, err
						}

						//Increase score counters
						killedBy := containerStatusPlayer.State.Terminated.Message
						reqLogger.Info("Player is killed", "Death", player.Name, "Kill", killedBy)
						increaseKill(battlefield, killedBy)
						increaseDeath(battlefield, player.Name)
						
					} else {

						//If time is up, delete pod
						if timeExpired {
							reqLogger.Info("Time is up - deleting pod", "Pod.Name", found.Name)
							err := r.client.Delete(ctx, found)
							if err != nil {
								reqLogger.Error(err, "Error deleting pod", "Pod.Name", found.Name)
								return reconcile.Result{}, err
							}
						}
					}
				}

				//Update Battlefield resource status; TODO: Maybe check if update is required...
				err := r.client.Status().Update(ctx, battlefield)
				if err != nil {
					reqLogger.Error(err, "Error updating Battlefield")
					return reconcile.Result{}, err
				}
			}
			

		}
	}

	//***************************
	// Post init - start game
	//***************************
	if battlefield.Status.Phase == "init" {
		reqLogger.Info("Starting game", "Duration", battlefield.Spec.Duration, "HitFrequency", battlefield.Spec.HitFrequency, "Number of players", len(battlefield.Spec.Players))

		battlefield.Status.Phase = "started"
		t := metav1.Now()
		battlefield.Status.StartTime = &t

		err := r.client.Status().Update(ctx, battlefield)
		if err != nil {
			reqLogger.Error(err, "Error updating Battlefield")
		}
		//Update triggers a new request - let's return to avoid racing conditions
		return reconcile.Result{}, err
	}

	//***************************
	// Time Expired - update status
	//***************************
	if timeExpired && battlefield.Status.Phase != "done" {

		battlefield.Status.Phase = "done"
		t := metav1.Now()
		battlefield.Status.StopTime = &t

		reqLogger.Info("Game Over", "Duration", battlefield.Spec.Duration, "StartTime", battlefield.Status.StartTime, "StopTime", battlefield.Status.StopTime)

		err := r.client.Status().Update(ctx, battlefield)
		if err != nil {
			reqLogger.Error(err, "Error updating Battlefield")
			return reconcile.Result{}, err
		}
	}

	reqLogger.Info("Reconciling done")
	return reconcile.Result{}, nil
}

//Service definition for a player
func newServiceForPlayer(battlefield *rhtev1alpha1.Battlefield, player *rhtev1alpha1.Player) *corev1.Service {
	labels := map[string]string{
		"app":         battlefield.Name,
		"battlefield": battlefield.Name,
		"player":      player.Name,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      battlefield.Name + "-player-" + player.Name,
			Namespace: battlefield.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Selector: map[string]string{
				"player": player.Name,
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// Pod definition for a player
func newPodForPlayer(battlefield *rhtev1alpha1.Battlefield, player *rhtev1alpha1.Player) *corev1.Pod {
	labels := map[string]string{
		"app":         battlefield.Name,
		"battlefield": battlefield.Name,
		"player":      player.Name,
	}

	annotations := map[string]string{
		"sidecar.istio.io/inject": "true",
	}

	//List of targets for this pod - practically hservice names
	var targets []string
	for _, target := range battlefield.Spec.Players {
		if player.Name != target.Name {
			targets = append(targets, battlefield.Name+"-player-"+target.Name)
		}
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        battlefield.Name + "-player-" + player.Name,
			Namespace:   battlefield.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "player",
					Image:           player.Image,
					ImagePullPolicy: corev1.PullAlways,
					Env: []corev1.EnvVar{
						{
							Name:  "BATTLEFIELD_PLAYER_NAME",
							Value: player.Name,
						},
						{
							Name:  "BATTLEFIELD_PLAYER_URLS",
							Value: strings.Join(targets, ","),
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
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromInt(8080),
							},
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func increaseKill(battlefield *rhtev1alpha1.Battlefield, playerName string) {
	for index, playerStatus := range battlefield.Status.Scores {
		if playerName == playerStatus.Name {
			battlefield.Status.Scores[index].Kill++
			return
		}
	}
}

func increaseDeath(battlefield *rhtev1alpha1.Battlefield, playerName string) {
	for index, playerStatus := range battlefield.Status.Scores {
		if playerName == playerStatus.Name {
			battlefield.Status.Scores[index].Death++
			return
		}
	}
}

func setPlayerReady(battlefield *rhtev1alpha1.Battlefield, playerName string, ready bool) {
	for index, playerStatus := range battlefield.Status.Scores {
		if playerName == playerStatus.Name {
			battlefield.Status.Scores[index].Ready = ready
			return
		}
	}
}


func getContainerStatus(containerStatuses []corev1.ContainerStatus, containerName string) *corev1.ContainerStatus {
	//containerState := &corev1.ContainerState{}
	for _, containerStatus := range containerStatuses {
		if containerStatus.Name == containerName {
			return &containerStatus
		}
	}
	return nil
}
