package serializer

import (
	"fmt"
	"io/ioutil"

	"github.com/Berops/platform/proto/pb"
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/encoding/protojson"
)

// WriteProtobufToBinaryFile writes protocol buffer message to binary file
func WriteProtobufToBinaryFile(message proto.Message, filename string) error {
	data, err := proto.Marshal(message) //serialize protocol buffer data
	if err != nil {
		return fmt.Errorf("cannot marshal proto message to binary: %w", err)
	}

	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("cannot write binary data to file: %w", err)
	}

	return nil
}

// ReadProtobufFromBinaryFile reads protocol buffer message from binary file
func ReadProtobufFromBinaryFile(message proto.Message, filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("cannot read binary data from file: %w", err)
	}

	err = proto.Unmarshal(data, message)
	if err != nil {
		return fmt.Errorf("cannot unmarshal binary to protobuf message: %w", err)
	}

	return nil
}

// WriteProtobufToJSONFile writes protocol buffer message to JSON file
func WriteProtobufToJSONFile(message proto.Message, filename string) error {
	data, err := ProtobufToJSON(message)
	if err != nil {
		return fmt.Errorf("cannot marshal proto message to JSON: %w", err)
	}

	err = ioutil.WriteFile(filename, []byte(data), 0644)
	if err != nil {
		return fmt.Errorf("cannot write JSON data to file: %w", err)
	}

	return nil
}

// ReadProtobufFromJSONFile reads protocol buffer message from json file
func ReadProtobufFromJSONFile(project *pb.Project, filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("cannot read json data from file: %w", err)
	}

	err = protojson.Unmarshal(data, project)
	if err != nil {
		return fmt.Errorf("cannot unmarshal json to protobuf message: %w", err)
	}

	return nil
}
