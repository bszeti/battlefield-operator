oc delete deployment battlefield-operator

oc delete battlefields --all
oc delete vs -l battlefield
oc delete service -l battlefield
oc delete pod -l battlefield

oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_health.yaml

operator-sdk up local 
