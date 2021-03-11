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

type RawWrapper struct {
	Proto interface{} `json:"proto,inline"`
	Raw   interface{} `json:"raw"`
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

func removeSerialized(c firestore.Client, txn *firestore.Transaction, collection string, state ProtoIdentifiable) error {
	query := c.Collection(collection).Where(DataPathID, "==", state.GetId())
	snaps, err := txn.Documents(query).GetAll()
	if err != nil {
		return err
	}
	for _, snap := range snaps {
		if err := txn.Delete(snap.Ref); err != nil {
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
	for _, snap := range snaps {
		if err := txn.Delete(snap.Ref); err != nil {
			return err
		}
	}
	return nil
}

func Serialize(txn *firestore.Transaction, collection CollectionName, state protoreflect.ProtoMessage) error {

	if identifiable, ok := state.(ProtoIdentifiable); ok {
		return serializeID(txn, string(collection), identifiable)
	}
	if named, ok := state.(ProtoNamed); ok {
		return serializeNamed(txn, string(collection), named)
	}
	return fmt.Errorf("serialize must contain proto with Name or ID fields")
}

func serializeID(txn *firestore.Transaction, collection string, state ProtoIdentifiable) error {

	raw, err := proto.Marshal(state)
	if err != nil {
		return err
	}
	RawObj := &RawWrapper{
		Proto: state,
		Raw:   raw,
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
		return err
	}
	return nil
}

func serializeNamed(txn *firestore.Transaction, collection string, state ProtoNamed) error {

	raw, err := proto.Marshal(state)
	if err != nil {
		return err
	}
	RawObj := &RawWrapper{
		Proto: state,
		Raw:   raw,
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
		return err
	}
	return nil
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
