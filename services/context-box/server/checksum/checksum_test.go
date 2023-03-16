package checksum

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// TestCalculateChecksum tests whether the output of CalculateCheckum is what
// we expect.
func TestCalculateChecksum(t *testing.T) {
	testCases := []struct {
		desc string
		in   string
		out  string
	}{
		{
			desc: "example short input checksum",
			in:   "0123",
			out:  "dec6c931b413074d13e11c60be35954a34cb2088b312c88e1d1a15d8ce64b561",
		},
		{
			desc: "example fuller manifest checksum",
			in: `name: ExampleManifest

# providers field is used for defining the providers
# every supported provider has an example in this input manifest
providers:
  gcp:
    - name: gcp0
      credentials: |
        {
          "type": "service_account",
          "project_id": "claudie-testing-123456",
          "private_key_id": "the id",
		  # this is not a real private key
          "private_key": "-----BEGIN PRIVATE KEY-----\nU2FsdGVkX1+f/IRYiYZl3JGsfaWyiAhLr9bBcwzGmc5jgqxNV/fQYPc1zYOs6wZ3aIzRazZg77bkixIepFbHlPy3u59x8dllSoCkKtTN7ycCTq6JpVO82q2x9DOu8JLu9scvUnSp4x13wc8dUAQgevIBjyx85ND8fGhe+V/q47fkTdwtFyH+VuTDMEMmMnSruLS6ujhjrItg6AAe4YQEs3UychtAUiqCql1C8I8r5LaA4dMXaoVJdkrKdUaY/uULP8zmgDmseAr3LUQeA8FT64PNA96ECFo8OqMHL+XD3Q6aARJn1Aw7CFeZtVQva47P6/sGv/gf/1nT3BHRPepy7dCYG9tvWAfoHkF8Zc/6Tefcb8Du0H/K3UHwJRNbJBr0/oPLCE9ImK6Q1p9eHLCMcUFFgUSJStxZwDXpHbhiZqBuS7e96thvvuNgbGju0sJ9/h/HTQJnA5mHsX5jOQeIP6GkVtbmk8sUGygl9lqcR/gQI1wVcAIHg+oaOEMA8Dc24m5R+xknSUZTb85tc1dk99PdkH9JbK+oCo2Htqqp+U7dBjJDPllaag8xN2NHNmGZhLel97FjuM0gwFaqP5pSdiFecBjA59PGo4+tSgz2KtqNNYSVRFnhidbjU/HdMFQLVtpaDqfhLm5E+hy/MOQYXJbsam0sNiZnP2r3yJ5vk7FkNRuB+1B9pruV8Lp6lad3ux3g+ShIJNJ1MEnnZDM1h07N9PBoQ2+G86fnqqVHTihAAX8RbG9Hy2ngUNjDl4P6FpEQoJ5Z51rbaGHAgOAs+o60j6+Z+ytVSQdYVhijPRCApCTxpghBlBi8nax3tzlN7PdCF4ZzGCo2AI+FmLHXl6E09YPYn/O+b3kQpD71hyQRVrRs1shxB554FLpwt6l9lGC/NDSZTTYPzBP3u+G5kxvRM1gGLnZ9BbylZhV9ePgHron/mMiIi8VhlEv0XVdRU5tGkbpsobyrYVUVCe+ABrLIqH2r6JhVmRGxZB00H+d9QcrnZlYh/Jb4FuuDqENXJtaa7tBqaC3Gu15AlurNymjrXIvRkuHV3duFJOdgPL5xWJQDJ0wawFsKcoWvua8ijCKMDSuNIsl8vy8XLkqX/jyKUsI1QPgoo4tycOn0YCga4heekHy2xXIB6hAR7bWx2LykDT2l/erwjg5Zk3HLPAeeQg3AzqCTnL1zUHInmB78XpurHhBU5Gf7efeDqUGFK5qkqbWXyj4BHnBwKYFo2lXAw9CY4AtL+ItVoHIDAbsANNBzjC9h1KAwpcjTl0OiA94zjEovT4ubIrMOIImGO/UxfhEHCdksc7IaH8ktDz0v+H3drFsbWsLBqBzqMKc/BWiOaDCGGScCpwZS+ZZZ/9OPXBjUmIVl7hHpkENnfWq7ZpA8pUwCZdkJYHTg8eS8\n-----END PRIVATE KEY-----\n",
          "client_email": "sth-compute@developer.gserviceaccount.com",
          "client_id": "the id",
          "auth_uri": "https://accounts.google.com/o/oauth2/auth",
          "token_uri": "https://oauth2.googleapis.com/token",
          "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
          "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/id-compute%40developer.gserviceaccount.com"
        }
      gcp_project: claudie-testing-123456
  oci:
    - name: oci-1
	  key_fingerprint: 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00
	  # this is not a real private key
      private_key: |
        -----BEGIN PRIVATE KEY-----
		U2FsdGVkX1+tKewBIdexBlaas5fVOk2Vkz+Evm3JcG9OwLiPKa
        7FAkXo8Qe54xpPdGbTXKNYwJUSeCzKW94jrvEZTmZ9BU+A7lsN
        STKEDXgC3xyFKJa7qAduQSsGpbN7TCUYtsAtGIg2gagWFqXpnu
        H89QWupQLn7u42RgUveEoR9zkPwNSzOziTLbPot6lGG9OH1fgS
        sgEz4dncZlsOznAHtChVXXTMQ+Rf4bFl/7uYz4G2jhR9EVnj1g
        cI5C8OKj8YBVESEBZ/LmHcrnqEwNrSXCR6SLXd/NMWnuAH7Chm
        zb6awouJ0dGqC8N4/YFBLxZUxSqV237SbVQj4+3Dggm/cQn/oi
        akNHJ7aW+WZuxliOj2Kf1XTb6W68N80XWQABOw3fRLUtqixeJe
        Hm6IpD+mPODhWqy5Pbm/k1WdzI+QGawYV7GKEeCW+joHK7CK/L
        uX38pPU246ez8TnnDDVy1VOf4y2/xEDrWEPNUckHTBR969m0QV
        nCQC8NuYCPpukaSOnyv9Od3bZB4lE/+a4A4N2r8kD5oZ1XedsZ
        DnMIBkQurcjK3GBle+UXnH3plSBx9j9+KMl6ffk3exP+s6+ONw
        TjWc7AIC8lrSwa1PdXTC0NYYSMUgItG5LkgyQoyOtgeehvDxmJ
        9cY4P3JoX3q+N5cth5W0ApXazta5Sg9Z6cKb2snxkPW8MPM2Zs
        QzL/D2vr95YD1YRcs8ArWUqxpsnWU+o40ItGBVOGPN6/4x/A2x
        BxDLWI1i4OOQbquYZloo8T5fx2BpoK4yfY9sEDH9Z/ZQ9qcHhh
        0QeZL54R7sW5uWy3wuC0I2TaJLxDwFP2yIUOJ+vOIUr5rZ9zmu
        3HyoaCbTdbVmwBY6rp8PakSDx9B1Li9lNwjuEmFVrTsXmPmfQR
        ff5BJcT2oc0A7Mx55BQKNtR/EMU7OVUu0Zqr6Y9A0uw9w/DjUe
        nlwr0JogT7QzNZNqRhZL0FNdjd6/7BziSWEkmPb9Ukgtus1ZlD
        m2Uhw29zvhd+IgrUqNsBSUKCRmk2YFse9vXvAEbP42hNxESlat
        iubSX4XacLClvElQPC7+4y4MYC905faE5tlGJYeZtPGFyac4pi
        8lXUIFtNO7sDAzX66Zw652cQHcVIXhEzgriULR+rPndHIKJ9YK
        GV5aoDt0zpbwa2EobMY5nIvaBXxARLi/ikknEKszYMayjLusNQ
        XqKKoK4T91Bv4/LkrP8bCwGGIC5Mg1E0qjNwYj2ZDPVaZyHKdq
        Hpb+gcULywMpYILsfgbq/+QDTOtQ+pNqAMwdF7xAS/2uqUR16X
        N7v8Gq+QC+LzVvANp4r4xwQyyiSrfokoh6WAoFs/U5ENa77akc
        bfthDc5XbAxCbh7z5OI5ojALOyZoHuZSTZ2sug9doED+7zd0V2
        z+n/Npq2
        -----END PRIVATE KEY-----
      tenancy_ocid: ocid1.tenancy.oc1..WHATEVER
      user_ocid: ocid1.user.oc1..WHATEVER
      compartment_ocid: ocid1.compartment.oc1..WHATEVER

# nodepools field is used for defining the nodepool spec
# you can think of them as a blueprints, not actual nodepools that will be created
nodePools:
  # dynamic nodepools are created by claudie
  dynamic:
    # control nodes
    - name: control-gcp
      providerSpec:
        name: gcp0
        region: europe-west6
        zone: europe-west6-a
      count: 2
      server_type: e2-medium
      image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
      disk_size: 50
    - name: compute-gcp
      providerSpec:
        name: gcp0
        region: europe-west6
        zone: europe-west6-a
      count: 2
      server_type: e2-small
      image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
      disk_size: 50
    - name: control-oci
      providerSpec:
        name: oci-1
        region: eu-frankfurt-1
        zone: hsVQ:EU-FRANKFURT-1-AD-1
      count: 2
      server_type: VM.Standard2.1
      # ubuntu minimal
      # https://docs.oracle.com/en-us/iaas/images/image/674d0b41-aee8-4c0b-bf99-9e100d90f241/
      image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
      disk_size: 50
    - name: compute-oci
      providerSpec:
        name: oci-1
        region: eu-frankfurt-1
        zone: hsVQ:EU-FRANKFURT-1-AD-1
      count: 2
      server_type: VM.Standard2.1
      # ubuntu minimal
      # https://docs.oracle.com/en-us/iaas/images/image/674d0b41-aee8-4c0b-bf99-9e100d90f241/
      image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
      disk_size: 50

# kubernetes field is used to define the k8s clusters
# here we define two clusters, dev and prod
kubernetes:
  clusters:
    - name: og-cluster
      version: v1.22.0
      network: 192.168.2.0/24
      # we can reuse same nodepool spec and claudie will create new nodes
      pools:
        control:
          - control-gcp
          - control-oci
        compute:
          - compute-gcp
          - compute-oci
`,
			out: "325f04bb5560e14e0b162ea3a70aa3368182fff4d433622f3aba20a7837803a8",
		},
		{
			desc: "test case containing unprintable characters",
			in:   " unprintable ",
			out:  "9969c5f9e2813e38773d38cb64a898e691423b2c5848c9b3a82f0a14d0f92f7b",
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			want, err := hex.DecodeString(tC.out)
			if err != nil {
				t.Errorf("Unexpected error: %q", err)
			}

			if got := CalculateChecksum(tC.in); !bytes.Equal(got, want) {
				t.Errorf("Unexpected output for %q: expected %+x, actual %+x",
					tC.desc, want, got)
			}
		})
	}
}

// TestCompareChecksums tests whether the output of CompareCheckums is what we
// expectEquals
func TestCompareChecksums(t *testing.T) {
	testCases := []struct {
		desc string
		in1  []byte
		in2  []byte
		out  bool
	}{
		{
			desc: "false - example short input checksum",
			in1:  []byte("0123"),
			in2:  []byte("987543987654543786543029835908643"),
			out:  false,
		},
		{
			desc: "false - test case containing unprintable characters",
			in1:  []byte(" unprintable characters follow "),
			in2:  []byte("098098987987876876765765756654463"),
			out:  false,
		},
		{
			desc: "true - test case containing unprintable characters",
			in1:  []byte(" sth then unprintable "),
			in2:  []byte(" sth then unprintable "),
			out:  true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			want := tC.out

			if got := Equals(tC.in1, tC.in2); want != got {
				t.Errorf("Unexpected output for %q: expected %t, actual %t",
					tC.desc, want, got)
			}
		})
	}
}
