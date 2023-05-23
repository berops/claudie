package claudieDB

import (
	"fmt"
	"log"
	"testing"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/proto/pb"
	"github.com/stretchr/testify/require"
)

var (
	desiredState *pb.Project = &pb.Project{
		Name: "TestProjectName",
	}
)

func TestSaveConfig(t *testing.T) {
	cm := ClaudieMongo{URL: envs.DatabaseURL}
	err := cm.Connect()
	require.NoError(t, err)
	err = cm.Init()
	require.NoError(t, err)
	defer func() {
		err := cm.Disconnect()
		if err != nil {
			log.Fatal(err)
		}
	}()
	conf := &pb.Config{DesiredState: desiredState, Name: "test-pb-config"}
	err = cm.SaveConfig(conf)
	require.NoError(t, err)
	fmt.Println("Config id: " + conf.Id)
	require.NotEmpty(t, conf.Id)
	err = cm.DeleteConfig(conf.Name, pb.IdType_NAME)
	require.NoError(t, err)
}

func TestUpdateTTL(t *testing.T) {
	cm := ClaudieMongo{URL: envs.DatabaseURL}
	err := cm.Connect()
	require.NoError(t, err)
	err = cm.Init()
	require.NoError(t, err)
	defer func() {
		err := cm.Disconnect()
		if err != nil {
			log.Fatal(err)
		}
	}()
	conf := &pb.Config{DesiredState: desiredState, Name: "test-pb-config", BuilderTTL: 1000, SchedulerTTL: 1000}
	err = cm.SaveConfig(conf)
	require.NoError(t, err)
	err = cm.UpdateBuilderTTL(conf.Name, 500)
	require.NoError(t, err)
	err = cm.UpdateSchedulerTTL(conf.Name, 200)
	require.NoError(t, err)
	conf, err = cm.GetConfig(conf.Name, pb.IdType_NAME)
	require.NoError(t, err)
	require.EqualValues(t, 500, conf.BuilderTTL)
	require.EqualValues(t, 200, conf.SchedulerTTL)
	err = cm.DeleteConfig(conf.Name, pb.IdType_NAME)
	require.NoError(t, err)
}

func TestSaveWorkflow(t *testing.T) {
	cm := ClaudieMongo{URL: envs.DatabaseURL}
	err := cm.Connect()
	require.NoError(t, err)
	err = cm.Init()
	require.NoError(t, err)
	conf := &pb.Config{DesiredState: desiredState, Name: "test-pb-config"}
	err = cm.SaveConfig(conf)
	require.NoError(t, err)
	err = cm.UpdateAllStates("test-pb-config", map[string]*pb.Workflow{"foo": {
		Stage:       pb.Workflow_KUBER,
		Status:      pb.Workflow_DONE,
		Description: "Test",
	}})
	require.NoError(t, err)
	c1, err := cm.getByNameFromDB(conf.Name)
	require.NoError(t, err)
	require.Equal(t, c1.State["foo"].Description, "Test")
	err = cm.UpdateAllStates(conf.Name, map[string]*pb.Workflow{"foo": {
		Stage:       pb.Workflow_NONE,
		Status:      pb.Workflow_DONE,
		Description: "Test1",
	}})
	require.NoError(t, err)
	c2, err := cm.getByNameFromDB(conf.Name)
	require.NoError(t, err)
	require.Equal(t, c2.State["foo"].Description, "Test1")
	err = cm.DeleteConfig(conf.Name, pb.IdType_NAME)
	require.NoError(t, err)
	err = cm.Disconnect()
	require.NoError(t, err)
}
