operator-sdk build quay.io/bszeti/battlefield-operator
docker push quay.io/bszeti/battlefield-operator
oc delete deployment battlefield-operator
oc create -f deploy/operator.yaml

oc delete battlefield --all
oc delete vs  -l battlefield=demofield
oc delete service  -l battlefield=demofield
oc delete pods -l battlefield=demofield
oc create -f deploy/crds/rhte_v1alpha1_battlefield_cr_health.yaml
oc get pods

