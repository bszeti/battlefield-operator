oc delete battlefields --all
oc delete pod -l battlefield
oc delete vs --all
oc delete service -l battlefield

oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_health.yaml

# operator-sdk up local 
