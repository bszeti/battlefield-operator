# oc create -f deploy/crds/rhte_v1alpha1_battlefield_crd.yaml

oc delete battlefields --all
oc delete pod --all
oc delete vs --all
oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr.yaml
operator-sdk up local 
