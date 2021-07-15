package wireguardian

import (
	"context"
	"log"

	"github.com/Berops/platform/proto/pb"
)

func BuildVPN(c pb.WireguardianServiceClient, req *pb.BuildVPNRequest) (*pb.BuildVPNResponse, error) {
	res, err := c.BuildVPN(context.Background(), req)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("VPN was successfully built")
	return res, nil
}
