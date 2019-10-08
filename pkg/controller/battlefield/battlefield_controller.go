package battlefield

import (
	"context"
	"io/ioutil"
	"net/http"
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
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	//istio "istio.io/api/networking/v1alpha3"
	istio "github.com/aspenmesh/istio-client-go/pkg/apis/networking/v1alpha3"
	istiov1alpha3 "istio.io/api/networking/v1alpha3"
)

var log = logf.Log.WithName("controller_battlefield")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

var netClient = &http.Client{
	Timeout: time.Second * 1,
}

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

	// TODO:REMOVE
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
					Name:          player.Name,
					Kill:          0,
					Death:         0,
					CurrentHealth: player.MaxHealth,
				})
		}

		//Init times - if this is a restart
		battlefield.Status.StartTime = nil
		battlefield.Status.StopTime = nil

		err := r.client.Status().Update(ctx, battlefield)
		if err != nil {
			reqLogger.Error(err, "Error updating Battlefield")
		}
		//The update will trigger a new request. It's better to return.
		return reconcile.Result{}, err

	}
	//check if time is expired
	if battlefield.Status.StartTime != nil {
		timeExpired = time.Now().After(battlefield.Status.StopTime.Time)
		//timeExpired = time.Since(battlefield.Status.StartTime.Time) > time.Duration(battlefield.Spec.Duration)*time.Second
	}

	//reqLogger.Info("Players", "Num of players", len(battlefield.Spec.Players), "Names", battlefield.Spec.Players )

	//***********************
	// Manage services
	//***********************
	for _, player := range battlefield.Spec.Players {
		playerService := newServiceForPlayer(battlefield, &player)
		shieldService := newFakeServiceForPlayer(battlefield, &player, "shield")
		disqualifiedService := newFakeServiceForPlayer(battlefield, &player, "disqualified")

		services:= []*corev1.Service{playerService, shieldService, disqualifiedService}

		
		for _,service := range services {

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

			//TODO: In case of error, the battlefield status may not be updated. Ideally we should build status independently from respawn actions.

			//check if container "player" is terminated - update scores and delete the pod to stop any sidecars
			containerStatusPlayer := getContainerStatus(found.Status.ContainerStatuses, "player")
			if containerStatusPlayer != nil { //It's expected to have a "player" container...
				setPlayerReady(battlefield, player.Name, containerStatusPlayer.Ready)
				if containerStatusPlayer.Ready || timeExpired {
					setKilledBy(battlefield, player.Name, "")
				}

				//Only manage pod if it's not marked for deletetion yet
				if found.DeletionTimestamp == nil {

					deletePod := false;
					killedBy := "" //For players who can't terminate (aka Quarkus)
					if containerStatusPlayer.Ready {
						//Get current health from pod
						url := "http://" + strings.ToLower(found.Name) + "/api/status/currenthealth"
						response, err := netClient.Get(url)
						if err != nil {
							//reqLogger.Error(err, "Error getting health for player", "url", url)
						} else {

							defer response.Body.Close()
							data, _ := ioutil.ReadAll(response.Body)
							currentHealth, strerr := strconv.Atoi(string(data))
							if strerr == nil {
								//Update current health in status
								setCurrentHealth(battlefield, player.Name, currentHealth)
								reqLogger.Info("CurrentHealth", "player", player.Name, "CurrentHealth", currentHealth)
								//Quarkus can't terminate, we have to do it from here. 
								// if currentHealth==0 {
								// 	deletePod=true
									
								// 	url := "http://" + found.Name + "/api/status/killedby"
								// 	response, err := netClient.Get(url)
								// 	if err != nil {
								// 		//reqLogger.Error(err, "Error getting killedBy for player", "url", url)
								// 	} else {
								// 		defer response.Body.Close()
								// 		data, err := ioutil.ReadAll(response.Body)
								// 		if err==nil{
								// 			killedBy = string(data)
								// 			log.Info("Api killedBy:"+killedBy)
								// 		}
								// 	}
									
								// }
							}
						}
					}

					
					if deletePod || containerStatusPlayer.State.Terminated != nil {
						//TODO: Sometimes points are counted multiple times because of racing conditions. client.Delete doesn't return if Pod was deleted before.

						reqLogger.Info("Killed - deleting Pod", "Pod.Name", found.Name)
						err = r.client.Delete(ctx, found)
						if err != nil {
							reqLogger.Error(err, "Error deleting Pod", "Pod.Name", found.Name)
							return reconcile.Result{}, err
						}

						//Increase score counters
						if containerStatusPlayer.State.Terminated != nil && len(containerStatusPlayer.State.Terminated.Message)!=0 {
							killedBy = containerStatusPlayer.State.Terminated.Message
							log.Info("Termination log:"+killedBy)
						}
						reqLogger.Info("Player is killed", "Death", player.Name, "Kill", killedBy)
						increaseKill(battlefield, killedBy)
						increaseDeath(battlefield, player.Name)
						setKilledBy(battlefield, player.Name, killedBy)
						setCurrentHealth(battlefield, player.Name, 0)

					} else {

						//If time is up, delete pod
						if timeExpired {
							reqLogger.Info("Time is up - deleting pod", "Pod.Name", found.Name)
							err := r.client.Delete(ctx, found)
							if err != nil {
								reqLogger.Error(err, "Error deleting pod", "Pod.Name", found.Name)
								return reconcile.Result{}, err
							}
							setCurrentHealth(battlefield, player.Name, 0)
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


		//Take care of VirtualService for the player
		virtualService := newVirtualServiceForPlayer(battlefield, &player)
		if err := controllerutil.SetControllerReference(battlefield, virtualService, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		foundVirtualService := &istio.VirtualService{}
		err = r.client.Get(ctx, types.NamespacedName{Name: virtualService.Name, Namespace: virtualService.Namespace}, foundVirtualService)
		if err != nil {
			if errors.IsNotFound(err) {
				if !timeExpired { //Create missing VirtualService if time is not expired
					reqLogger.Info("Creating VirtualService", "VirtualService.Name", virtualService.Name)
					err := r.client.Create(ctx, virtualService)
					if err != nil {
						reqLogger.Error(err, "Error creating VirtualService", "VirtualService.Name", virtualService.Name)
						return reconcile.Result{}, err
					}
				}
			} else {
				reqLogger.Error(err, "Error reading VirtualService", "VirtualService.Name", virtualService.Name)
				return reconcile.Result{}, err
			}
		} else {
			//VirtualService is found and not deleted
			if foundVirtualService.DeletionTimestamp == nil {

				if !timeExpired {
					//TODO: Compare old and new VirtualService
					//reqLogger.Info("Updating VirtualService", "VirtualService.Name", virtualService.Name)
					foundVirtualService.Spec = virtualService.Spec
					err := r.client.Update(ctx, foundVirtualService)
					if err != nil {
						reqLogger.Error(err, "Error creating VirtualService", "virtualService", virtualService)
						return reconcile.Result{}, err
					}
				}
				//If time is up, delete foundVirtualService
				if timeExpired {
					reqLogger.Info("Time is up - deleting VirtualService", "VirtualService.Name", foundVirtualService.Name)
					err := r.client.Delete(ctx, foundVirtualService)
					if err != nil {
						reqLogger.Error(err, "Error deleting VirtualService", "VirtualService.Name", foundVirtualService.Name)
						return reconcile.Result{}, err
					}
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
		tend := metav1.NewTime(t.Add(time.Duration(battlefield.Spec.Duration) * time.Second))
		battlefield.Status.StartTime = &t
		battlefield.Status.StopTime = &tend

		err := r.client.Status().Update(ctx, battlefield)
		if err != nil {
			reqLogger.Error(err, "Error updating Battlefield")
			return reconcile.Result{}, err
		}
		//Update triggers a new request right away so it's ok to return here - we use this return to do a reconciliation when time expires
		return reconcile.Result{RequeueAfter: time.Duration(battlefield.Spec.Duration) * time.Second}, err
	}

	//***************************
	// Time Expired - update status
	//***************************
	if timeExpired && battlefield.Status.Phase != "done" {

		battlefield.Status.Phase = "done"
		// t := metav1.Now()
		// battlefield.Status.StopTime = &t

		reqLogger.Info("Game Over", "Duration", battlefield.Spec.Duration, "StartTime", battlefield.Status.StartTime, "StopTime", battlefield.Status.StopTime)

		err := r.client.Status().Update(ctx, battlefield)
		if err != nil {
			reqLogger.Error(err, "Error updating Battlefield")
			return reconcile.Result{}, err
		}

	}

	reqLogger.Info("Reconciling done")
	if !timeExpired {
		return reconcile.Result{RequeueAfter: time.Millisecond * 200}, nil
	}
	return reconcile.Result{}, nil
}

//Service definition for a player
func newServiceForPlayer(battlefield *rhtev1alpha1.Battlefield, player *rhtev1alpha1.Player) *corev1.Service {
	labels := map[string]string{
		"app":         strings.ToLower(battlefield.Name + "-" + player.Name)	,
		"battlefield": battlefield.Name,
		"player":      player.Name,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.ToLower(battlefield.Name + "-player-" + player.Name),
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
				"player":      strings.ToLower(player.Name),
				"battlefield": strings.ToLower(battlefield.Name),
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

//Service "shield" and "disqualified"
func newFakeServiceForPlayer(battlefield *rhtev1alpha1.Battlefield, player *rhtev1alpha1.Player, serviceType string) *corev1.Service {
	labels := map[string]string{
		"app":         strings.ToLower(battlefield.Name + "-" + player.Name + "-" + serviceType),
		"battlefield": battlefield.Name,
		"player":      strings.ToLower(player.Name),
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.ToLower(battlefield.Name + "-player-" + player.Name + "-" + serviceType),
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
				"player":      serviceType,
				"battlefield": battlefield.Name,
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

//Virtual Service for shield
func newVirtualServiceForPlayer(battlefield *rhtev1alpha1.Battlefield, player *rhtev1alpha1.Player) *istio.VirtualService {
	labels := map[string]string{
		"app":         strings.ToLower(battlefield.Name + "-" + player.Name),
		"battlefield": battlefield.Name,
		"player":      strings.ToLower(player.Name),
	}

	resourceNameForPlayer := strings.ToLower(battlefield.Name + "-player-" + player.Name)

	vs := istio.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceNameForPlayer,
			Namespace: battlefield.Namespace,
			Labels:    labels,
		},
		Spec: istio.VirtualServiceSpec{
			VirtualService: istiov1alpha3.VirtualService{
				Hosts: []string{
					resourceNameForPlayer,
				},
				Http: []*istiov1alpha3.HTTPRoute{},
			},
		},
	}

	//Create rules for disqualified players
	for _, disqualifiedPlayer := range battlefield.Spec.Players {
		if disqualifiedPlayer.Disqualified {
			disqualifiedRule := &istiov1alpha3.HTTPRoute{

				Route: []*istiov1alpha3.HTTPRouteDestination{
					&istiov1alpha3.HTTPRouteDestination{
						Destination: &istiov1alpha3.Destination{
							
							Host: strings.ToLower(battlefield.Name + "-player-" + disqualifiedPlayer.Name+"-disqualified"),
						},
					},
				},

				Fault: &istiov1alpha3.HTTPFaultInjection{
					Abort: &istiov1alpha3.HTTPFaultInjection_Abort{
						Percentage: &istiov1alpha3.Percent{
							Value: 100.0,
						},
						ErrorType: &istiov1alpha3.HTTPFaultInjection_Abort_HttpStatus{
							HttpStatus: 505,
						},
					},
				},

				Match:  []*istiov1alpha3.HTTPMatchRequest{
					&istiov1alpha3.HTTPMatchRequest{
						SourceLabels: map[string]string{
							"player": strings.ToLower(disqualifiedPlayer.Name),
						},
					},
				},
			}
			vs.Spec.VirtualService.Http = append(vs.Spec.VirtualService.Http, disqualifiedRule)
		}
	
	}

	//Player has shield
	if player.Shield {

		shieldRule := &istiov1alpha3.HTTPRoute{

			Route: []*istiov1alpha3.HTTPRouteDestination{
				&istiov1alpha3.HTTPRouteDestination{
					Destination: &istiov1alpha3.Destination{
						Host: resourceNameForPlayer+"-shield", 
					},
				},
			},

			Fault: &istiov1alpha3.HTTPFaultInjection{
				Abort: &istiov1alpha3.HTTPFaultInjection_Abort{
					Percentage: &istiov1alpha3.Percent{
						Value: 100.0,
					},
					ErrorType: &istiov1alpha3.HTTPFaultInjection_Abort_HttpStatus{
						HttpStatus: 504,
					},
				},
			},

			Match: []*istiov1alpha3.HTTPMatchRequest{
				&istiov1alpha3.HTTPMatchRequest{
					Uri: &istiov1alpha3.StringMatch{
						MatchType: &istiov1alpha3.StringMatch_Prefix{
							Prefix: "/api/hit",
						},
					},
				},
			},
		}

		vs.Spec.VirtualService.Http = append(vs.Spec.VirtualService.Http, shieldRule)
	} else {

		//Add rule for not ready player to return 200
		for _, playerStatus := range battlefield.Status.Scores {
			//log.Info("notReadyRule","player.Name",player.Name, "playerStatus.Name", playerStatus.Name, "playerStatus.Ready", playerStatus.Ready)
			if player.Name == playerStatus.Name && playerStatus.Ready == false {
				//log.Info("Adding rule...")
				notReadyRule := &istiov1alpha3.HTTPRoute{

					Route: []*istiov1alpha3.HTTPRouteDestination{
						&istiov1alpha3.HTTPRouteDestination{
							Destination: &istiov1alpha3.Destination{
								Host: resourceNameForPlayer,
							},
						},
					},
		
					Fault: &istiov1alpha3.HTTPFaultInjection{
						Abort: &istiov1alpha3.HTTPFaultInjection_Abort{
							Percentage: &istiov1alpha3.Percent{
								Value: 100.0,
							},
							ErrorType: &istiov1alpha3.HTTPFaultInjection_Abort_HttpStatus{
								HttpStatus: 205,
							},
						},
					},
				}
				vs.Spec.VirtualService.Http = append(vs.Spec.VirtualService.Http, notReadyRule)	
			}
		}
	}


	//Default rule to hit the actual service
	defaultRule := &istiov1alpha3.HTTPRoute{
		Route: []*istiov1alpha3.HTTPRouteDestination{
			&istiov1alpha3.HTTPRouteDestination{
				Destination: &istiov1alpha3.Destination{
					Host: resourceNameForPlayer,
				},
			},
		},
	}

	vs.Spec.VirtualService.Http = append(vs.Spec.VirtualService.Http, defaultRule)

	return &vs

}


// Pod definition for a player
func newPodForPlayer(battlefield *rhtev1alpha1.Battlefield, player *rhtev1alpha1.Player) *corev1.Pod {
	labels := map[string]string{
		"app":         strings.ToLower(battlefield.Name + "-" + player.Name),
		"battlefield": battlefield.Name,
		"player":      strings.ToLower(player.Name),
	}

	annotations := map[string]string{
		"sidecar.istio.io/inject": "true",
	}

	//List of targets for this pod - practically service names
	var targets []string
	for _, target := range battlefield.Spec.Players {
		if player.Name != target.Name {
			targets = append(targets, strings.ToLower(battlefield.Name+"-player-"+target.Name))
		}
	}

	hitPeriodMs := strconv.Itoa(battlefield.Spec.HitFrequency * 1000)
	hitPeriodDuration:= strconv.Itoa(battlefield.Spec.HitFrequency)+".0s"
	if (player.Type == "cheater") {
		hitPeriodMs = strconv.Itoa(battlefield.Spec.HitFrequency * 250)
		hitPeriodDuration = strconv.FormatFloat(float64(battlefield.Spec.HitFrequency)/4.0,'f',2,64)+"s"
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        strings.ToLower(battlefield.Name + "-player-" + player.Name),
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
					//ImagePullPolicy: corev1.PullIfNotPresent,
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
							Name:  "BATTLEFIELD_HIT_PERIOD_MS",
							Value: hitPeriodMs,
						},
						{
							Name:  "BATTLEFIELD_HIT_PERIOD_DURATION",
							Value: hitPeriodDuration,
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"memory": resource.MustParse("256Mi"),
						},
						Limits: corev1.ResourceList{
							"memory": resource.MustParse("512Mi"),
						},

					},
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/health",
								Port: intstr.FromInt(8080),
							},
						},
						SuccessThreshold: 1,
						FailureThreshold: 1,
						PeriodSeconds: 1,
						InitialDelaySeconds: 1,
						TimeoutSeconds: 1,
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

func setKilledBy(battlefield *rhtev1alpha1.Battlefield, playerName string, killedBy string) {
	for index, playerStatus := range battlefield.Status.Scores {
		if playerName == playerStatus.Name {
			battlefield.Status.Scores[index].KilledBy = killedBy
			return
		}
	}
}

func setCurrentHealth(battlefield *rhtev1alpha1.Battlefield, playerName string, currentHealth int) {
	for index, playerStatus := range battlefield.Status.Scores {
		if playerName == playerStatus.Name {
			battlefield.Status.Scores[index].CurrentHealth = currentHealth
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
