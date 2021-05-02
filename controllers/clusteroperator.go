

package controllers
/*
//go:generate go run -mod=vendor ../vendor/github.com/go-bindata/go-bindata/go-bindata/ -nometadata -pkg $GOPACKAGE -ignore=bindata.go  ../manifests/0000_91_kubernetes-nmstate-operator_06_clusteroperator.cr.yaml
//go:generate gofmt -s -l -w bindata.go

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"

	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
)

// StatusReason is a MixedCaps string representing the reason for a
// status condition change.
type StatusReason string

const (
	clusterOperatorName = "kubernetes-nmstate"

	// OperatorDisabled represents a Disabled ClusterStatusConditionTypes
	OperatorDisabled osconfigv1.ClusterStatusConditionType = "Disabled"

	// ReasonEmpty is an empty StatusReason
	ReasonEmpty StatusReason = ""

	// ReasonExpected indicates that the operator is behaving as expected
	ReasonExpected StatusReason = "AsExpected"

	// ReasonComplete the deployment of required resources is complete
	ReasonComplete StatusReason = "DeployComplete"

	// ReasonSyncing means that resources are deploying
	ReasonSyncing StatusReason = "SyncingResources"

	// ReasonInvalidConfiguration indicates that the configuration is invalid
	ReasonInvalidConfiguration StatusReason = "InvalidConfiguration"

	// ReasonDeployTimedOut indicates that the deployment timedOut
	ReasonDeployTimedOut StatusReason = "DeployTimedOut"

	// ReasonDeploymentCrashLooping indicates that the deployment is crashlooping
	ReasonDeploymentCrashLooping StatusReason = "DeploymentCrashLooping"

	// ReasonUnsupported is an unsupported StatusReason
	ReasonUnsupported StatusReason = "UnsupportedPlatform"
)

// defaultStatusConditions returns the default set of status conditions for the
// ClusterOperator resource used on first creation of the ClusterOperator.
func defaultStatusConditions() []osconfigv1.ClusterOperatorStatusCondition {
	return []osconfigv1.ClusterOperatorStatusCondition{
		setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, "", ""),
		setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
		setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, "", ""),
		setStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", ""),
		setStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
	}
}



// relatedObjects returns the current list of ObjectReference's for the
// ClusterOperator objects's status.
func relatedObjects() []osconfigv1.ObjectReference {
	return []osconfigv1.ObjectReference{
		{
			Group:    "",
			Resource: "namespaces",
			Name:     componentNamespace,
		},
		{
			Group:    "cluster-hosted-net-services.openshift.io",
			Resource: "config",
			Name:     "",
		},
		// TODO, add the operand namespace if needed
	}
}

// operandVersions returns the current list of OperandVersions for the
// ClusterOperator objects's status.
func operandVersions(version string) []osconfigv1.OperandVersion {
	operandVersions := []osconfigv1.OperandVersion{}

	if version != "" {
		operandVersions = append(operandVersions, osconfigv1.OperandVersion{
			Name:    "operator",
			Version: version,
		})
	}

	return operandVersions
}

// createClusterOperator creates the ClusterOperator and updates its status.
func (r *ConfigReconciler) createClusterOperator() (*osconfigv1.ClusterOperator, error) {
	b, err := Asset("../manifests/0000_91_cluster-hosted-net-services-operator_06_clusteroperator.cr.yaml")
	if err != nil {
		return nil, err
	}

	codecs := serializer.NewCodecFactory(r.Scheme)
	obj, _, err := codecs.UniversalDeserializer().Decode(b, nil, nil)
	if err != nil {
		return nil, err
	}

	defaultCO, ok := obj.(*osconfigv1.ClusterOperator)
	if !ok {
		return nil, fmt.Errorf("could not convert deserialized asset into ClusterOperoator")
	}

	return r.OSClient.ConfigV1().ClusterOperators().Create(context.Background(), defaultCO, metav1.CreateOptions{})
}

// ensureClusterOperator makes sure that the CO exists
func (r *ConfigReconciler) ensureClusterOperator(clusterHostedConfig *clusterhostednetservicesopenshiftiov1beta1.Config) error {
	co, err := r.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		co, err = r.createClusterOperator()
	}
	if err != nil {
		return err
	}

	// if the CO has been created with the manifest then we need to update the ownership
	if len(co.ObjectMeta.OwnerReferences) == 0 {
		err = controllerutil.SetControllerReference(clusterHostedConfig, co, r.Scheme)
		if err != nil {
			return err
		}

		co, err = r.OSClient.ConfigV1().ClusterOperators().Update(context.Background(), co, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	needsUpadate := false
	if !equality.Semantic.DeepEqual(co.Status.RelatedObjects, relatedObjects()) {
		needsUpadate = true
		co.Status.RelatedObjects = relatedObjects()
	}
	if !equality.Semantic.DeepEqual(co.Status.Versions, operandVersions(r.ReleaseVersion)) {
		needsUpadate = true
		co.Status.Versions = operandVersions(r.ReleaseVersion)
	}
	if len(co.Status.Conditions) == 0 {
		needsUpadate = true
		co.Status.Conditions = defaultStatusConditions()
	}

	if needsUpadate {
		_, err = r.OSClient.ConfigV1().ClusterOperators().UpdateStatus(context.Background(), co, metav1.UpdateOptions{})
	}
	return err
}


func (r *ConfigReconciler) getClusterOperator() (*osconfigv1.ClusterOperator, error) {
	existing, err := r.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to get clusterOperator %q: %v", clusterOperatorName, err)
	}

	return existing, nil
}

// setStatusCondition initalizes and returns a ClusterOperatorStatusCondition
func setStatusCondition(conditionType osconfigv1.ClusterStatusConditionType,
	conditionStatus osconfigv1.ConditionStatus, reason string,
	message string) osconfigv1.ClusterOperatorStatusCondition {
	return osconfigv1.ClusterOperatorStatusCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

//syncStatus applies the new condition to the CBO ClusterOperator object.
func (r *ConfigReconciler) syncStatus(co *osconfigv1.ClusterOperator, conds []osconfigv1.ClusterOperatorStatusCondition) error {
	for _, c := range conds {
		v1helpers.SetStatusCondition(&co.Status.Conditions, c)
	}

	if len(co.Status.Versions) < 1 {
		r.Log.Info("updating ClusterOperator Status Versions field")
		co.Status.Versions = operandVersions(r.ReleaseVersion)
	}

	_, err := r.OSClient.ConfigV1().ClusterOperators().UpdateStatus(context.Background(), co, metav1.UpdateOptions{})
	return err
}

func (r *ConfigReconciler) updateCOStatus(newReason StatusReason, msg, progressMsg string) error {

	co, err := r.getClusterOperator()
	if err != nil {
		r.Log.Error(err, "failed to get or create ClusterOperator")
		return err
	}
	conds := defaultStatusConditions()
	switch newReason {
	case ReasonUnsupported:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(OperatorDisabled, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonExpected), "Operational"))
	case ReasonSyncing:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(newReason), progressMsg))
	case ReasonComplete:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(newReason), progressMsg))
	case ReasonInvalidConfiguration, ReasonDeployTimedOut:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonEmpty), ""))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionTrue, string(newReason), progressMsg))
	case ReasonDeploymentCrashLooping:
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, string(newReason), msg))
		v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, string(newReason), progressMsg))
	}

	return r.syncStatus(co, conds)
}

func SetCOInDisabledState(osClient osclientset.Interface, version string) error {
	//The CVO should have created the CO
	co, err := osClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})

	if err != nil {
		return fmt.Errorf("failed to get clusterOperator %q: %v", clusterOperatorName, err)
	}

	conds := defaultStatusConditions()
	v1helpers.SetStatusCondition(&conds, setStatusCondition(OperatorDisabled, osconfigv1.ConditionTrue, string(ReasonUnsupported), "Nothing to do on this Platform"))
	v1helpers.SetStatusCondition(&conds, setStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, string(ReasonExpected), "Operational"))

	for _, c := range conds {
		v1helpers.SetStatusCondition(&co.Status.Conditions, c)
	}
	co.Status.Versions = operandVersions(version)
	co.Status.RelatedObjects = relatedObjects()

	_, err = osClient.ConfigV1().ClusterOperators().UpdateStatus(context.Background(), co, metav1.UpdateOptions{})
	return err
}

*/