package state

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	firestorepb "google.golang.org/genproto/googleapis/firestore/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"k8s.io/klog"
)

type CollectionName string

const (
	DataPathRaw  = "Raw"
	DataPathID   = "Proto.Id"
	DataPathName = "Proto.Name"

	// Firestore collection names
	// CollectionCustomer - the collection to serialize stripe customer proto messages to
	CollectionCustomer CollectionName = "customers"
	// CollectionSubscription - the collection to serialize stripe subscription proto messages to
	CollectionSubscription CollectionName = "subscriptions"
	// CollectionWorkspace - the collection to serialize stripe subscription workspace proto messages to
	CollectionWorkspace CollectionName = "workspaces"
	// CollectionPlan - the collection to serialize stripe plans proto messages to
	CollectionPlan CollectionName = "plans"
	// CollectionProducts - the collection to serialize stripe product proto messages to
	CollectionProducts CollectionName = "products"
)

var projectID string

func init() {

	if project := os.Getenv("PROJECT_ID"); project != "" {
		projectID = project
	} else {
		// Retrieve the projectID from the metadata service
		req, err := http.NewRequest(http.MethodGet, "http://metadata.google.internal/computeMetadata/v1/project/project-id", nil)
		if err != nil {
			klog.Fatalf("Failed to determine google project id: %v", err)
		}
		req.Header.Add("Metadata-Flavor", "Google")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			klog.Fatalf("Failed to request google project id: %v", err)
		}
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			klog.Fatalf("Could not parse google project id: %v", err)
		}
		projectID = string(data)
	}
}

type ProtoState struct {
	Proto    interface{}              `json:"proto,inline"`
	Raw      interface{}              `json:"raw"`
	Parent   *firestore.DocumentRef   `json:"parent,omitempty"`
	Children []*firestore.DocumentRef `json:"children,omitempty"`
}

type ProtoIdentifiable interface {
	protoreflect.ProtoMessage
	GetId() string
}

type ProtoNamed interface {
	protoreflect.ProtoMessage
	GetName() string
}

type ProtoSubscription interface {
	protoreflect.ProtoMessage
	GetSubscription() string
}

func GetDatabasePath() string {
	return fmt.Sprintf("projects/%s/databases/(default)", projectID)
}

func RemoveSerialized(c firestore.Client, txn *firestore.Transaction, collection CollectionName, state protoreflect.ProtoMessage) error {

	if identifiable, ok := state.(ProtoIdentifiable); ok {
		return removeSerialized(c, txn, string(collection), identifiable)
	}
	if named, ok := state.(ProtoNamed); ok {
		return removeSerializedNamed(c, txn, string(collection), named)
	}
	return fmt.Errorf("serialize must contain proto with Name or ID fields")
}

func DocumentChildren(txn *firestore.Transaction, snap *firestore.DocumentSnapshot) ([]*firestore.DocumentRef, error) {
	var ret []*firestore.DocumentRef
	var state ProtoState
	err := snap.DataTo(&state)
	if err != nil {
		return nil, err
	}
	for _, child := range state.Children {
		ret = append(ret, child)
		childSnap, err := txn.Get(child)
		if err != nil {
			return nil, err
		}
		grandchildren, err := DocumentChildren(txn, childSnap)
		if err != nil {
			return nil, err
		}
		ret = append(ret, grandchildren...)
	}
	return ret, nil
}

func removeSerialized(c firestore.Client, txn *firestore.Transaction, collection string, state ProtoIdentifiable) error {
	query := c.Collection(collection).Where(DataPathID, "==", state.GetId())
	snaps, err := txn.Documents(query).GetAll()
	if err != nil {
		return err
	}
	// Gather Children
	var refs []*firestore.DocumentRef
	for _, snap := range snaps {
		refs = append(refs, snap.Ref)
		children, err := DocumentChildren(txn, snap)
		if err != nil {
			return err
		}
		refs = append(refs, children...)
	}
	for _, ref := range refs {
		if err := txn.Delete(ref); err != nil {
			return err
		}
	}
	return nil
}

func removeSerializedNamed(c firestore.Client, txn *firestore.Transaction, collection string, state ProtoNamed) error {
	query := c.Collection(collection).Where(DataPathName, "==", state.GetName())
	snaps, err := txn.Documents(query).GetAll()
	if err != nil {
		return err
	}
	// Gather Children
	var refs []*firestore.DocumentRef
	for _, snap := range snaps {
		refs = append(refs, snap.Ref)
		children, err := DocumentChildren(txn, snap)
		if err != nil {
			return err
		}
		refs = append(refs, children...)
	}
	for _, ref := range refs {
		if err := txn.Delete(ref); err != nil {
			return err
		}
	}
	return nil
}

/*
Serialize returns the full path of the stored document or an error
*/
func Serialize(txn *firestore.Transaction, collection CollectionName, state protoreflect.ProtoMessage, parent *firestore.DocumentRef, children []*firestore.DocumentRef) (string, error) {

	if identifiable, ok := state.(ProtoIdentifiable); ok {
		return serializeID(txn, string(collection), identifiable, parent, children)
	}
	if named, ok := state.(ProtoNamed); ok {
		return serializeNamed(txn, string(collection), named, parent, children)
	}
	return "", fmt.Errorf("serialize must contain proto with Name or ID fields")
}

func serializeID(txn *firestore.Transaction, collection string, state ProtoIdentifiable, parent *firestore.DocumentRef, children []*firestore.DocumentRef) (string, error) {

	raw, err := proto.Marshal(state)
	if err != nil {
		return "", err
	}
	RawObj := &ProtoState{
		Proto:    state,
		Raw:      raw,
		Parent:   parent,
		Children: children,
	}

	// remarshal into generic to support inline options for json marshalling
	// genericMap, err := generic(RawObj)
	// if err != nil {
	// 	return err
	// }

	ref := &firestore.DocumentRef{
		Parent: &firestore.CollectionRef{
			ID:   collection,
			Path: fmt.Sprintf("%s/documents/%s", GetDatabasePath(), collection),
		},
		ID:   state.GetId(),
		Path: fmt.Sprintf("%s/documents/%s/%s", GetDatabasePath(), collection, state.GetId()),
	}
	if err := txn.Set(ref, RawObj); err != nil {
		return "", err
	}
	return ref.Path, nil
}

func serializeNamed(txn *firestore.Transaction, collection string, state ProtoNamed, parent *firestore.DocumentRef, children []*firestore.DocumentRef) (string, error) {

	raw, err := proto.Marshal(state)
	if err != nil {
		return "", err
	}
	RawObj := &ProtoState{
		Proto:    state,
		Raw:      raw,
		Parent:   parent,
		Children: children,
	}

	docName := state.GetName()
	// Named protos are weak and we may want to prefix on subscription if it exists
	if subscriptioned, ok := state.(ProtoSubscription); ok {
		docName = fmt.Sprintf("%s-%s", subscriptioned.GetSubscription(), docName)
	}

	ref := &firestore.DocumentRef{
		Parent: &firestore.CollectionRef{
			ID:   collection,
			Path: fmt.Sprintf("%s/documents/%s", GetDatabasePath(), collection),
		},
		ID:   docName,
		Path: fmt.Sprintf("%s/documents/%s/%s", GetDatabasePath(), collection, docName),
	}
	if err := txn.Set(ref, RawObj); err != nil {
		return "", err
	}
	return ref.Path, nil
}

func Deserialize(doc *firestorepb.Document, msg protoreflect.ProtoMessage) error {

	if raw, ok := doc.Fields[DataPathRaw]; ok {
		err := proto.Unmarshal(raw.GetBytesValue(), msg)
		if err != nil {
			return err
		}
	}
	return nil
}
