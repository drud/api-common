package state

import (
	"fmt"

	"cloud.google.com/go/firestore"
	firestorepb "google.golang.org/genproto/googleapis/firestore/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type rawWrapper struct {
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

func (s *Manager) removeSerialized(txn *firestore.Transaction, collection string, state ProtoIdentifiable) error {
	query := s.firestore.Collection(collection).Where("Proto.Id", "==", state.GetId())

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

func (s *Manager) removeSerializedNamed(txn *firestore.Transaction, collection string, state ProtoNamed) error {
	query := s.firestore.Collection(collection).Where("Proto.Name", "==", state.GetName())
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

func (s *Manager) Serialize(txn *firestore.Transaction, collection string, state protoreflect.ProtoMessage) error {
	if identifiable, ok := state.(ProtoIdentifiable); ok {
		return s.serializeID(txn, collection, identifiable)
	}
	if named, ok := state.(ProtoNamed); ok {
		return s.serializeNamed(txn, collection, named)
	}
	return fmt.Errorf("serialize must contain proto with Name or ID fields")
}

func (s *Manager) serializeID(txn *firestore.Transaction, collection string, state ProtoIdentifiable) error {

	raw, err := proto.Marshal(state)
	if err != nil {
		return err
	}
	RawObj := &rawWrapper{
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
			Path: fmt.Sprintf("%s/documents/%s", getDatabasePath(), collection),
		},
		ID:   state.GetId(),
		Path: fmt.Sprintf("%s/documents/%s/%s", getDatabasePath(), collection, state.GetId()),
	}
	if err := txn.Set(ref, RawObj); err != nil {
		return err
	}
	return nil
}

func (s *Manager) serializeNamed(txn *firestore.Transaction, collection string, state ProtoNamed) error {

	raw, err := proto.Marshal(state)
	if err != nil {
		return err
	}
	RawObj := &rawWrapper{
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
			Path: fmt.Sprintf("%s/documents/%s", getDatabasePath(), collection),
		},
		ID:   docName,
		Path: fmt.Sprintf("%s/documents/%s/%s", getDatabasePath(), collection, docName),
	}
	if err := txn.Set(ref, RawObj); err != nil {
		return err
	}
	return nil
}

func Deserialize(doc *firestorepb.Document, msg protoreflect.ProtoMessage) error {

	if raw, ok := doc.Fields["Raw"]; ok {
		err := proto.Unmarshal(raw.GetBytesValue(), msg)
		if err != nil {
			return err
		}
	}
	return nil
}
