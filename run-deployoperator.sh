set -e
operator-sdk build quay.io/bszeti/battlefield-operator
docker push quay.io/bszeti/battlefield-operator

oc delete battlefields --all
oc delete vs -l battlefield
oc delete service -l battlefield
oc delete pod -l battlefield

oc delete deployment battlefield-operator || true
oc create -f deploy/operator.yaml

#oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_health.yaml
#oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_java.yaml
oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_quarkusmix.yaml

oc get pods -l name=battlefield-operator
