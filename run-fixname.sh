oc delete battlefields --all
oc delete pod --all
oc delete vs --all
oc delete service --all

oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_fixname.yaml

operator-sdk up local 
