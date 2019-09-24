# Add CRD to the cluster - if it's not there yet. 
#oc get crd battlefields.rhte.demojam.battlefield
#oc create -f crds/rhte_v1alpha1_battlefield_crd.yaml

#Deploy operator in the current namespace
oc create -f service_account.yaml
oc create -f role.yaml
oc create -f role_binding.yaml
oc create -f operator.yaml

# Create games
# oc create -f crds/rhte_v1alpha1_battlefield_cr.yaml

# Watch logs
# oc logs battlefield-operator-5748f4b8c6-5f6wq -f

# Check battlefield status
# oc get bf -oyaml my-bn2qn 
