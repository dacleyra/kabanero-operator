#!/bin/bash

set -x pipefail

RELEASE="${RELEASE:-0.3.5}"
KABANERO_SUBSCRIPTIONS_YAML="${KABANERO_SUBSCRIPTIONS_YAML:-https://github.com/kabanero-io/kabanero-operator/releases/download/$RELEASE/kabanero-subscriptions.yaml}"
KABANERO_CUSTOMRESOURCES_YAML="${KABANERO_CUSTOMRESOURCES_YAML:-https://github.com/kabanero-io/kabanero-operator/releases/download/$RELEASE/kabanero-customresources.yaml}"
SLEEP_LONG="${SLEEP_LONG:-5}"
SLEEP_SHORT="${SLEEP_SHORT:-2}"

### Check prereqs

# oc installed
if ! which oc; then
  printf "oc client is required, please install oc client in your PATH.\nhttps://mirror.openshift.com/pub/openshift-v4/clients/oc/latest"
  exit 1
fi

# oc logged in
if ! oc whoami; then
  printf "Not logged in. Please login as a cluster-admin."
  exit 1
fi

# oc version
OCMIN="4.2.0"
OCVER=$(oc version -o=yaml | grep  gitVersion | head -1 | sed -nre 's/^[^0-9]*(([0-9]+\.)*[0-9]+).*/\1/p')
OCHEAD=$(printf "$OCMIN\n$OCVER" | sort -V | head -n 1)
if [ "$OCMIN" != "$OCHEAD" ]; then
  printf "oc client version is $OCVER. Minimum oc client version required is $OCMIN.\nhttps://mirror.openshift.com/pub/openshift-v4/clients/oc/latest".
  exit 1
fi


# Knative Eventing no longer required
CRDS=(eventing.knative.dev checlusters.org.eclipse.che)


# Github Sources no longer required
# Github Sources
oc delete --all-namespaces=true githubsources.sources.eventing.knative.dev --all
echo "Waiting for githubsources instances to be deleted...."
LOOP_COUNT=0
while [ `oc get githubsources.sources.eventing.knative.dev --all-namespaces | wc -l` -gt 0 ]
do
  sleep $SLEEP_LONG
  LOOP_COUNT=`expr $LOOP_COUNT + 1`
  if [ $LOOP_COUNT -gt 10 ] ; then
    echo "Timed out waiting for githubsources.sources.eventing.knative.dev instances to be deleted"
    exit 1
  fi
done
oc delete -f https://github.com/knative/eventing-contrib/releases/download/v0.9.0/github.yaml


# The SMCP and SMMR required by Serverless is now managed by its own operator
oc delete -f $KABANERO_CUSTOMRESOURCES_YAML --ignore-not-found --selector kabanero.io/install=21-cr-servicemeshmemberrole,kabanero.io/namespace!=true
oc delete -f $KABANERO_CUSTOMRESOURCES_YAML --ignore-not-found --selector kabanero.io/install=20-cr-servicemeshcontrolplane,kabanero.io/namespace!=true


# Remove the old tekton dashboard
# Tekton Dashboard
oc delete --ignore-not-found -f https://github.com/tektoncd/dashboard/releases/download/v0.2.1/openshift-tekton-dashboard-release.yaml
oc delete --ignore-not-found -f https://github.com/tektoncd/dashboard/releases/download/v0.2.1/openshift-tekton-webhooks-extension-release.yaml


# Serving CR does not upgrade correctly, so delete
oc delete -f $KABANERO_CUSTOMRESOURCES_YAML --ignore-not-found --selector kabanero.io/install=22-cr-knative-serving,kabanero.io/namespace!=true


### Subscriptions

# Args: subscription metadata.name, namespace
# Deletes Subscription, InstallPlan, CSV
unsubscribe () {
	# Get InstallPlan
	INSTALL_PLAN=$(oc get subscription $1 -n $2 --output=jsonpath={.status.installPlanRef.name})
	
	# Get CluserServiceVersion
	CSV=$(oc get subscription $1 -n $2 --output=jsonpath={.status.installedCSV})
	
	# Delete Subscription 
	oc delete subscription $1 -n $2

	# Delete the InstallPlan
	oc delete installplan $INSTALL_PLAN -n $2
	
	# Delete the Installed ClusterServiceVersion
	oc delete clusterserviceversion $CSV -n $2
	
	# Waat for the Copied ClusterServiceVersions to cleanup
	if [ -n "$CSV" ] ; then
		while [ `oc get clusterserviceversions $CSV --all-namespaces | wc -l` -gt 0 ]
		do
			sleep 5
			LOOP_COUNT=`expr $LOOP_COUNT + 1`
			if [ $LOOP_COUNT -gt 10 ] ; then
					echo "Timed out waiting for Copied ClusterServiceVersions $CSV to be deleted"
					exit 1
			fi
		done
	fi
}


# Knative eventing no longer required
unsubscribe knative-eventing-operator openshift-operators

# Che replaced by CRWS
unsubscribe eclipse-che kabanero

# Ensure CSV Cleanup in all namespaces
OPERATORS=(knative-eventing-operator )
for OPERATOR in "${OPERATORS[@]}"
do
  CSV=$(oc --all-namespaces=true get csv --output=jsonpath={.items[*].metadata.name} | tr " " "\n" | grep ${OPERATOR} | head -1)
  if [ -n "${CSV}" ]; then
    oc --all-namespaces=true delete csv --field-selector=metadata.name="${CSV}"
  fi
done

# Delete CRDs
for CRD in "${CRDS[@]}"
do
  unset CRDNAMES
  CRDNAMES=$(oc get crds -o=jsonpath='{range .items[*]}{"\n"}{@.metadata.name}{end}' | grep '.*\.'$CRD)
  if [ -n "$CRDNAMES" ]; then
    echo $CRDNAMES | xargs -n 1 oc delete crd
  fi
done


### Some CSVs did not upgrade correctly, this was necessary
# oc delete csv servicemeshoperator.v1.0.2 -n kabanero 
# oc delete csv servicemeshoperator.v1.0.2 -n openshift-monitoring
# oc delete csv servicemeshoperator.v1.0.2 -n openshift-operator-lifecycle-manager