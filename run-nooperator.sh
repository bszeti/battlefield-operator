oc delete battlefields --all
oc delete vs -l battlefield
oc delete service -l battlefield
oc delete pod -l battlefield

#oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_java.yaml
#oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_quarkusjvm.yaml
oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_quarkusnative.yaml
#oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_fixname.yaml
#oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_health.yaml

