package main

import (
	"claudie/shared/envs"
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	inboundAdapters "claudie/microservices/frontend/adapters/inbound"
	outboundAdapters "claudie/microservices/frontend/adapters/outbound"
	"claudie/microservices/frontend/domain/usecases"
)

const (
	// k8sNotificationsReceiverPort is the port at which the cli service listens for notification
	// from the k8s-sidecar service about changes in the directory containing the manifest files.
	k8sNotificationsReceiverPort= 50059
)

func main( ) {
	if err := run( ); err != nil {
		log.Fatal( ).Msg(err.Error( ))}
}

func run( ) error {
	contextBoxConnector, err := outboundAdapters.NewContextBoxConnector(envs.ContextBoxUri)
	if err != nil {
		return err}

	usecases := &usecases.Usecases{
		ContextBox: contextBoxConnector,
	}

	k8sNotificationsReceiver, err := inboundAdapters.NewK8sNotificationsReceiver(usecases)
	if err != nil {
		return err}

	waitGroup, waitGroupContext := errgroup.WithContext(context.Background( ))

	waitGroup.Go(func( ) error {
		log.Info( ).Msgf("Listening for notifications from k8s-sidecar at port: %v", k8sNotificationsReceiverPort)
		return k8sNotificationsReceiver.Start("0.0.0.0", k8sNotificationsReceiverPort)
	})

	waitGroup.Go(func( ) error {
		shutdownSignalChan := make(chan os.Signal, 1)
		signal.Notify(shutdownSignalChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(shutdownSignalChan)

		var err error

		select {
			case <- waitGroupContext.Done( ):
				err= waitGroupContext.Err( )

			case shutdownSignal := <- shutdownSignalChan:
				log.Info( ).Msgf("Received program shutdown signal %v", shutdownSignal)
				err= errors.New("program interruption signal")
		}

		// First shutdown the HTTP server to block any incoming connections.
		// And wait for all the go-routines to finish their work.
		performGracefulShutdown := func( ) {
			log.Info( ).Msg("Gracefully shutting down K8sSidecarNotificationsReceiver and ContextBoxCommunicator")

			if err := k8sNotificationsReceiver.Stop( ); err != nil {
				log.Error( ).Msgf("Failed to gracefully shutdown K8sNotificationsReceiver: %w", err)}

			if err := contextBoxConnector.Stop( ); err != nil {
				log.Error( ).Msgf("Failed to gracefully shutdown ContextBoxConnector: %w", err)}

		}
		performGracefulShutdown( )

		return err
	})

	return waitGroup.Wait( )
}