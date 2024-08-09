package userdata

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/confidential-containers/cloud-api-adaptor/src/cloud-providers/azure"
)

var testAgentConfig string = `server_addr = 'unix:///run/kata-containers/agent.sock'
guest_components_procs = 'none'
`

var testDaemonConfig string = `{
	"pod-network": {
		"podip": "10.244.0.19/24",
		"pod-hw-addr": "0e:8f:62:f3:81:ad",
		"interface": "eth0",
		"worker-node-ip": "10.224.0.4/16",
		"tunnel-type": "vxlan",
		"routes": [
			{
				"Dst": "",
				"GW": "10.244.0.1",
				"Dev": "eth0"
			}
		],
		"mtu": 1500,
		"index": 1,
		"vxlan-port": 8472,
		"vxlan-id": 555001,
		"dedicated": false
	},
	"pod-namespace": "default",
	"pod-name": "nginx-866fdb5bfb-b98nw",
	"tls-server-key": "-----BEGIN PRIVATE KEY-----\n....\n-----END PRIVATE KEY-----\n",
	"tls-server-cert": "-----BEGIN CERTIFICATE-----\n....\n-----END CERTIFICATE-----\n",
	"tls-client-ca": "-----BEGIN CERTIFICATE-----\n....\n-----END CERTIFICATE-----\n"
}
`

var testAuthJson string = `{
	"auths":{}
}
`

var testCDHConfig string = `socket = 'unix:///run/confidential-containers/cdh.sock'
credentials = []

[kbc]
name = 'cc_kbc'
url = 'http://1.2.3.4:8080'
kbs_cert = """
-----BEGIN CERTIFICATE-----
MIIFTDCCAvugAwIBAgIBADBGBgkqhkiG9w0BAQowOaAPMA0GCWCGSAFlAwQCAgUA
oRwwGgYJKoZIhvcNAQEIMA0GCWCGSAFlAwQCAgUAogMCATCjAwIBATB7MRQwEgYD
VQQLDAtFbmdpbmVlcmluZzELMAkGA1UEBhMCVVMxFDASBgNVBAcMC1NhbnRhIENs
YXJhMQswCQYDVQQIDAJDQTEfMB0GA1UECgwWQWR2YW5jZWQgTWljcm8gRGV2aWNl
czESMBAGA1UEAwwJU0VWLU1pbGFuMB4XDTIzMDEyNDE3NTgyNloXDTMwMDEyNDE3
NTgyNlowejEUMBIGA1UECwwLRW5naW5lZXJpbmcxCzAJBgNVBAYTAlVTMRQwEgYD
VQQHDAtTYW50YSBDbGFyYTELMAkGA1UECAwCQ0ExHzAdBgNVBAoMFkFkdmFuY2Vk
IE1pY3JvIERldmljZXMxETAPBgNVBAMMCFNFVi1WQ0VLMHYwEAYHKoZIzj0CAQYF
K4EEACIDYgAExmG1ZbuoAQK93USRyZQcsyobfbaAEoKEELf/jK39cOVJt1t4s83W
XM3rqIbS7qHUHQw/FGyOvdaEUs5+wwxpCWfDnmJMAQ+ctgZqgDEKh1NqlOuuKcKq
2YAWE5cTH7sHo4IBFjCCARIwEAYJKwYBBAGceAEBBAMCAQAwFwYJKwYBBAGceAEC
BAoWCE1pbGFuLUIwMBEGCisGAQQBnHgBAwEEAwIBAzARBgorBgEEAZx4AQMCBAMC
AQAwEQYKKwYBBAGceAEDBAQDAgEAMBEGCisGAQQBnHgBAwUEAwIBADARBgorBgEE
AZx4AQMGBAMCAQAwEQYKKwYBBAGceAEDBwQDAgEAMBEGCisGAQQBnHgBAwMEAwIB
CDARBgorBgEEAZx4AQMIBAMCAXMwTQYJKwYBBAGceAEEBEDDhCejDUx6+dlvehW5
cmmCWmTLdqI1L/1dGBFdia1HP46MC82aXZKGYSutSq37RCYgWjueT+qCMBE1oXDk
d1JOMEYGCSqGSIb3DQEBCjA5oA8wDQYJYIZIAWUDBAICBQChHDAaBgkqhkiG9w0B
AQgwDQYJYIZIAWUDBAICBQCiAwIBMKMDAgEBA4ICAQACgCai9x8DAWzX/2IelNWm
ituEBSiq9C9eDnBEckQYikAhPasfagnoWFAtKu/ZWTKHi+BMbhKwswBS8W0G1ywi
cUWGlzigI4tdxxf1YBJyCoTSNssSbKmIh5jemBfrvIBo1yEd+e56ZJMdhN8e+xWU
bvovUC2/7Dl76fzAaACLSorZUv5XPJwKXwEOHo7FIcREjoZn+fKjJTnmdXce0LD6
9RHr+r+ceyE79gmK31bI9DYiJoL4LeGdXZ3gMOVDR1OnDos5lOBcV+quJ6JujpgH
d9g3Sa7Du7pusD9Fdap98ocZslRfFjFi//2YdVM4MKbq6IwpYNB+2PCEKNC7SfbO
NgZYJuPZnM/wViES/cP7MZNJ1KUKBI9yh6TmlSsZZOclGJvrOsBZimTXpATjdNMt
cluKwqAUUzYQmU7bf2TMdOXyA9iH5wIpj1kWGE1VuFADTKILkTc6LzLzOWCofLxf
onhTtSDtzIv/uel547GZqq+rVRvmIieEuEvDETwuookfV6qu3D/9KuSr9xiznmEg
xynud/f525jppJMcD/ofbQxUZuGKvb3f3zy+aLxqidoX7gca2Xd9jyUy5Y/83+ZN
bz4PZx81UJzXVI9ABEh8/xilATh1ZxOePTBJjN7lgr0lXtKYjV/43yyxgUYrXNZS
oLSG2dLCK9mjjraPjau34Q==
-----END CERTIFICATE-----
"""
`

var testAAConfig string = `[token_configs]
[token_configs.coco_as]
url = 'http://127.0.0.1:8080'

[token_configs.kbs]
url = 'http://127.0.0.1:8080'
cert = """
-----BEGIN CERTIFICATE-----
MIIDljCCAn6gAwIBAgIUR/UNh13GFam4emgludtype/S9BIwDQYJKoZIhvcNAQEL
BQAwdTELMAkGA1UEBhMCQ04xETAPBgNVBAgMCFpoZWppYW5nMREwDwYDVQQHDAhI
YW5nemhvdTERMA8GA1UECgwIQUFTLVRFU1QxFDASBgNVBAsMC0RldmVsb3BtZW50
MRcwFQYDVQQDDA5BQVMtVEVTVC1IVFRQUzAeFw0yNDAzMTgwNzAzNTNaFw0yNTAz
MTgwNzAzNTNaMHUxCzAJBgNVBAYTAkNOMREwDwYDVQQIDAhaaGVqaWFuZzERMA8G
A1UEBwwISGFuZ3pob3UxETAPBgNVBAoMCEFBUy1URVNUMRQwEgYDVQQLDAtEZXZl
bG9wbWVudDEXMBUGA1UEAwwOQUFTLVRFU1QtSFRUUFMwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQDfp1aBr6LiNRBlJUcDGcAbcUCPG6UzywtVIc8+comS
ay//gwz2AkDmFVvqwI4bdp/NUCwSC6ShHzxsrCEiagRKtA3af/ckM7hOkb4S6u/5
ewHHFcL6YOUp+NOH5/dSLrFHLjet0dt4LkyNBPe7mKAyCJXfiX3wb25wIBB0Tfa0
p5VoKzwWeDQBx7aX8TKbG6/FZIiOXGZdl24DGARiqE3XifX7DH9iVZ2V2RL9+3WY
05GETNFPKtcrNwTy8St8/HsWVxjAzGFzf75Lbys9Ff3JMDsg9zQzgcJJzYWisxlY
g3CmnbENP0eoHS4WjQlTUyY0mtnOwodo4Vdf8ZOkU4wJAgMBAAGjHjAcMBoGA1Ud
EQQTMBGCCWxvY2FsaG9zdIcEfwAAATANBgkqhkiG9w0BAQsFAAOCAQEAKW32spii
t2JB7C1IvYpJw5mQ5bhIlldE0iB5rwWvNbuDgPrgfTI4xiX5sumdHw+P2+GU9KXF
nWkFRZ9W/26xFrVgGIS/a07aI7xrlp0Oj+1uO91UhCL3HhME/0tPC6z1iaFeZp8Y
T1tLnafqiGiThFUgvg6PKt86enX60vGaTY7sslRlgbDr9sAi/NDSS7U1PviuC6yo
yJi7BDiRSx7KrMGLscQ+AKKo2RF1MLzlJMa1kIZfvKDBXFzRd61K5IjDRQ4HQhwX
DYEbQvoZIkUTc1gBUWDcAUS5ztbJg9LCb9WVtvUTqTP2lGuNymOvdsuXq+sAZh9b
M9QaC1mzQ/OStg==
-----END CERTIFICATE-----
"""
`

var testPolicyConfig string = `package agent_policy

import future.keywords.in
import future.keywords.every

import input

# Default values, returned by OPA when rules cannot be evaluated to true.
default CopyFileRequest := false
default CreateContainerRequest := false
default CreateSandboxRequest := true
default DestroySandboxRequest := true
default ExecProcessRequest := false
default GetOOMEventRequest := true
default GuestDetailsRequest := true
default OnlineCPUMemRequest := true
default PullImageRequest := true
default ReadStreamRequest := false
default RemoveContainerRequest := true
default RemoveStaleVirtiofsShareMountsRequest := true
default SignalProcessRequest := true
default StartContainerRequest := true
default StatsContainerRequest := true
default TtyWinResizeRequest := true
default UpdateEphemeralMountsRequest := true
default UpdateInterfaceRequest := true
default UpdateRoutesRequest := true
default WaitProcessRequest := true
default WriteStreamRequest := false
`

var testCheckSum = "52af3178dd7ad4bf551e629b84b45bfd1fbe1434b980120267181ae3575ea20ca9013b8eadf31d27eed7ff2552d500ef"
var cc_init_data = "YWxnb3JpdGhtID0gInNoYTM4NCIKdmVyc2lvbiA9ICIwLjEuMCIKCltkYXRhXQoiYWEudG9tbCIgPSAnJycKW3Rva2VuX2NvbmZpZ3NdClt0b2tlbl9jb25maWdzLmNvY29fYXNdCnVybCA9ICdodHRwOi8vMTI3LjAuMC4xOjgwODAnCgpbdG9rZW5fY29uZmlncy5rYnNdCnVybCA9ICdodHRwOi8vMTI3LjAuMC4xOjgwODAnCmNlcnQgPSAiIiIKLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURsakNDQW42Z0F3SUJBZ0lVUi9VTmgxM0dGYW00ZW1nbHVkdHlwZS9TOUJJd0RRWUpLb1pJaHZjTkFRRUwKQlFBd2RURUxNQWtHQTFVRUJoTUNRMDR4RVRBUEJnTlZCQWdNQ0Zwb1pXcHBZVzVuTVJFd0R3WURWUVFIREFoSQpZVzVuZW1odmRURVJNQThHQTFVRUNnd0lRVUZUTFZSRlUxUXhGREFTQmdOVkJBc01DMFJsZG1Wc2IzQnRaVzUwCk1SY3dGUVlEVlFRRERBNUJRVk10VkVWVFZDMUlWRlJRVXpBZUZ3MHlOREF6TVRnd056QXpOVE5hRncweU5UQXoKTVRnd056QXpOVE5hTUhVeEN6QUpCZ05WQkFZVEFrTk9NUkV3RHdZRFZRUUlEQWhhYUdWcWFXRnVaekVSTUE4RwpBMVVFQnd3SVNHRnVaM3BvYjNVeEVUQVBCZ05WQkFvTUNFRkJVeTFVUlZOVU1SUXdFZ1lEVlFRTERBdEVaWFpsCmJHOXdiV1Z1ZERFWE1CVUdBMVVFQXd3T1FVRlRMVlJGVTFRdFNGUlVVRk13Z2dFaU1BMEdDU3FHU0liM0RRRUIKQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUURmcDFhQnI2TGlOUkJsSlVjREdjQWJjVUNQRzZVenl3dFZJYzgrY29tUwpheS8vZ3d6MkFrRG1GVnZxd0k0YmRwL05VQ3dTQzZTaEh6eHNyQ0VpYWdSS3RBM2FmL2NrTTdoT2tiNFM2dS81CmV3SEhGY0w2WU9VcCtOT0g1L2RTTHJGSExqZXQwZHQ0TGt5TkJQZTdtS0F5Q0pYZmlYM3diMjV3SUJCMFRmYTAKcDVWb0t6d1dlRFFCeDdhWDhUS2JHNi9GWklpT1hHWmRsMjRER0FSaXFFM1hpZlg3REg5aVZaMlYyUkw5KzNXWQowNUdFVE5GUEt0Y3JOd1R5OFN0OC9Ic1dWeGpBekdGemY3NUxieXM5RmYzSk1Ec2c5elF6Z2NKSnpZV2lzeGxZCmczQ21uYkVOUDBlb0hTNFdqUWxUVXlZMG10bk93b2RvNFZkZjhaT2tVNHdKQWdNQkFBR2pIakFjTUJvR0ExVWQKRVFRVE1CR0NDV3h2WTJGc2FHOXpkSWNFZndBQUFUQU5CZ2txaGtpRzl3MEJBUXNGQUFPQ0FRRUFLVzMyc3BpaQp0MkpCN0MxSXZZcEp3NW1RNWJoSWxsZEUwaUI1cndXdk5idURnUHJnZlRJNHhpWDVzdW1kSHcrUDIrR1U5S1hGCm5Xa0ZSWjlXLzI2eEZyVmdHSVMvYTA3YUk3eHJscDBPaisxdU85MVVoQ0wzSGhNRS8wdFBDNnoxaWFGZVpwOFkKVDF0TG5hZnFpR2lUaEZVZ3ZnNlBLdDg2ZW5YNjB2R2FUWTdzc2xSbGdiRHI5c0FpL05EU1M3VTFQdml1QzZ5bwp5Smk3QkRpUlN4N0tyTUdMc2NRK0FLS28yUkYxTUx6bEpNYTFrSVpmdktEQlhGelJkNjFLNUlqRFJRNEhRaHdYCkRZRWJRdm9aSWtVVGMxZ0JVV0RjQVVTNXp0YkpnOUxDYjlXVnR2VVRxVFAybEd1TnltT3Zkc3VYcStzQVpoOWIKTTlRYUMxbXpRL09TdGc9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCiIiIgonJycKCiJjZGgudG9tbCIgID0gJycnCnNvY2tldCA9ICd1bml4Oi8vL3J1bi9jb25maWRlbnRpYWwtY29udGFpbmVycy9jZGguc29jaycKY3JlZGVudGlhbHMgPSBbXQoKW2tiY10KbmFtZSA9ICdjY19rYmMnCnVybCA9ICdodHRwOi8vMS4yLjMuNDo4MDgwJwprYnNfY2VydCA9ICIiIgotLS0tLUJFR0lOIENFUlRJRklDQVRFLS0tLS0KTUlJRlREQ0NBdnVnQXdJQkFnSUJBREJHQmdrcWhraUc5dzBCQVFvd09hQVBNQTBHQ1dDR1NBRmxBd1FDQWdVQQpvUnd3R2dZSktvWklodmNOQVFFSU1BMEdDV0NHU0FGbEF3UUNBZ1VBb2dNQ0FUQ2pBd0lCQVRCN01SUXdFZ1lEClZRUUxEQXRGYm1kcGJtVmxjbWx1WnpFTE1Ba0dBMVVFQmhNQ1ZWTXhGREFTQmdOVkJBY01DMU5oYm5SaElFTnMKWVhKaE1Rc3dDUVlEVlFRSURBSkRRVEVmTUIwR0ExVUVDZ3dXUVdSMllXNWpaV1FnVFdsamNtOGdSR1YyYVdObApjekVTTUJBR0ExVUVBd3dKVTBWV0xVMXBiR0Z1TUI0WERUSXpNREV5TkRFM05UZ3lObG9YRFRNd01ERXlOREUzCk5UZ3lObG93ZWpFVU1CSUdBMVVFQ3d3TFJXNW5hVzVsWlhKcGJtY3hDekFKQmdOVkJBWVRBbFZUTVJRd0VnWUQKVlFRSERBdFRZVzUwWVNCRGJHRnlZVEVMTUFrR0ExVUVDQXdDUTBFeEh6QWRCZ05WQkFvTUZrRmtkbUZ1WTJWawpJRTFwWTNKdklFUmxkbWxqWlhNeEVUQVBCZ05WQkFNTUNGTkZWaTFXUTBWTE1IWXdFQVlIS29aSXpqMENBUVlGCks0RUVBQ0lEWWdBRXhtRzFaYnVvQVFLOTNVU1J5WlFjc3lvYmZiYUFFb0tFRUxmL2pLMzljT1ZKdDF0NHM4M1cKWE0zcnFJYlM3cUhVSFF3L0ZHeU92ZGFFVXM1K3d3eHBDV2ZEbm1KTUFRK2N0Z1pxZ0RFS2gxTnFsT3V1S2NLcQoyWUFXRTVjVEg3c0hvNElCRmpDQ0FSSXdFQVlKS3dZQkJBR2NlQUVCQkFNQ0FRQXdGd1lKS3dZQkJBR2NlQUVDCkJBb1dDRTFwYkdGdUxVSXdNQkVHQ2lzR0FRUUJuSGdCQXdFRUF3SUJBekFSQmdvckJnRUVBWng0QVFNQ0JBTUMKQVFBd0VRWUtLd1lCQkFHY2VBRURCQVFEQWdFQU1CRUdDaXNHQVFRQm5IZ0JBd1VFQXdJQkFEQVJCZ29yQmdFRQpBWng0QVFNR0JBTUNBUUF3RVFZS0t3WUJCQUdjZUFFREJ3UURBZ0VBTUJFR0Npc0dBUVFCbkhnQkF3TUVBd0lCCkNEQVJCZ29yQmdFRUFaeDRBUU1JQkFNQ0FYTXdUUVlKS3dZQkJBR2NlQUVFQkVERGhDZWpEVXg2K2RsdmVoVzUKY21tQ1dtVExkcUkxTC8xZEdCRmRpYTFIUDQ2TUM4MmFYWktHWVN1dFNxMzdSQ1lnV2p1ZVQrcUNNQkUxb1hEawpkMUpPTUVZR0NTcUdTSWIzRFFFQkNqQTVvQTh3RFFZSllJWklBV1VEQkFJQ0JRQ2hIREFhQmdrcWhraUc5dzBCCkFRZ3dEUVlKWUlaSUFXVURCQUlDQlFDaUF3SUJNS01EQWdFQkE0SUNBUUFDZ0NhaTl4OERBV3pYLzJJZWxOV20KaXR1RUJTaXE5QzllRG5CRWNrUVlpa0FoUGFzZmFnbm9XRkF0S3UvWldUS0hpK0JNYmhLd3N3QlM4VzBHMXl3aQpjVVdHbHppZ0k0dGR4eGYxWUJKeUNvVFNOc3NTYkttSWg1amVtQmZydklCbzF5RWQrZTU2WkpNZGhOOGUreFdVCmJ2b3ZVQzIvN0RsNzZmekFhQUNMU29yWlV2NVhQSndLWHdFT0hvN0ZJY1JFam9abitmS2pKVG5tZFhjZTBMRDYKOVJIcityK2NleUU3OWdtSzMxYkk5RFlpSm9MNExlR2RYWjNnTU9WRFIxT25Eb3M1bE9CY1YrcXVKNkp1anBnSApkOWczU2E3RHU3cHVzRDlGZGFwOThvY1pzbFJmRmpGaS8vMllkVk00TUticTZJd3BZTkIrMlBDRUtOQzdTZmJPCk5nWllKdVBabk0vd1ZpRVMvY1A3TVpOSjFLVUtCSTl5aDZUbWxTc1paT2NsR0p2ck9zQlppbVRYcEFUamROTXQKY2x1S3dxQVVVellRbVU3YmYyVE1kT1h5QTlpSDV3SXBqMWtXR0UxVnVGQURUS0lMa1RjNkx6THpPV0NvZkx4ZgpvbmhUdFNEdHpJdi91ZWw1NDdHWnFxK3JWUnZtSWllRXVFdkRFVHd1b29rZlY2cXUzRC85S3VTcjl4aXpubUVnCnh5bnVkL2Y1MjVqcHBKTWNEL29mYlF4VVp1R0t2YjNmM3p5K2FMeHFpZG9YN2djYTJYZDlqeVV5NVkvODMrWk4KYno0UFp4ODFVSnpYVkk5QUJFaDgveGlsQVRoMVp4T2VQVEJKak43bGdyMGxYdEtZalYvNDN5eXhnVVlyWE5aUwpvTFNHMmRMQ0s5bWpqcmFQamF1MzRRPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQoiIiIKJycnCgoicG9saWN5LnJlZ28iID0gJycnCnBhY2thZ2UgYWdlbnRfcG9saWN5CgppbXBvcnQgZnV0dXJlLmtleXdvcmRzLmluCmltcG9ydCBmdXR1cmUua2V5d29yZHMuZXZlcnkKCmltcG9ydCBpbnB1dAoKIyBEZWZhdWx0IHZhbHVlcywgcmV0dXJuZWQgYnkgT1BBIHdoZW4gcnVsZXMgY2Fubm90IGJlIGV2YWx1YXRlZCB0byB0cnVlLgpkZWZhdWx0IENvcHlGaWxlUmVxdWVzdCA6PSBmYWxzZQpkZWZhdWx0IENyZWF0ZUNvbnRhaW5lclJlcXVlc3QgOj0gZmFsc2UKZGVmYXVsdCBDcmVhdGVTYW5kYm94UmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgRGVzdHJveVNhbmRib3hSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBFeGVjUHJvY2Vzc1JlcXVlc3QgOj0gZmFsc2UKZGVmYXVsdCBHZXRPT01FdmVudFJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IEd1ZXN0RGV0YWlsc1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IE9ubGluZUNQVU1lbVJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFB1bGxJbWFnZVJlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFJlYWRTdHJlYW1SZXF1ZXN0IDo9IGZhbHNlCmRlZmF1bHQgUmVtb3ZlQ29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgUmVtb3ZlU3RhbGVWaXJ0aW9mc1NoYXJlTW91bnRzUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgU2lnbmFsUHJvY2Vzc1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFN0YXJ0Q29udGFpbmVyUmVxdWVzdCA6PSB0cnVlCmRlZmF1bHQgU3RhdHNDb250YWluZXJSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBUdHlXaW5SZXNpemVSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVFcGhlbWVyYWxNb3VudHNSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVJbnRlcmZhY2VSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBVcGRhdGVSb3V0ZXNSZXF1ZXN0IDo9IHRydWUKZGVmYXVsdCBXYWl0UHJvY2Vzc1JlcXVlc3QgOj0gdHJ1ZQpkZWZhdWx0IFdyaXRlU3RyZWFtUmVxdWVzdCA6PSBmYWxzZQonJyc="

// Test server to simulate the metadata service
func startTestServer() *httptest.Server {
	// Create base64 encoded test data
	testUserDataString := base64.StdEncoding.EncodeToString([]byte("test data"))

	// Create a handler function for the desired path /metadata/instance/compute/userData
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Check that the request path is correct
		if r.URL.Path != "/metadata/instance/compute/userData" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		// Check that the request method is correct
		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}

		// Write the test data to the response
		if _, err := io.WriteString(w, testUserDataString); err != nil {
			http.Error(w, "Error writing response.", http.StatusNotFound)
		}
	}

	// Create a test server
	srv := httptest.NewServer(http.HandlerFunc(handler))

	fmt.Printf("Started metadata server at srv.URL: %s\n", srv.URL)

	return srv
}

// test server, serving plain text userData
func startTestServerPlainText() *httptest.Server {

	// Create a handler function for the desired path /metadata/instance/compute/userData
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Check that the request path is correct
		if r.URL.Path != "/metadata/instance/compute/userData" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		// Check that the request method is correct
		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}

		// Write the test data to the response
		if _, err := io.WriteString(w, "test data"); err != nil {
			http.Error(w, "Error writing response.", http.StatusNotFound)
		}
	}

	// Create a test server
	srv := httptest.NewServer(http.HandlerFunc(handler))

	fmt.Printf("Started metadata server at srv.URL: %s\n", srv.URL)

	return srv

}

// TestGetUserData tests the getUserData function
func TestGetUserData(t *testing.T) {
	// Start a temporary HTTP server for the test simulating
	// the Azure metadata service
	srv := startTestServer()
	defer srv.Close()

	// Create a context
	ctx := context.Background()

	// Send request to srv.URL at path /metadata/instance/compute/userData

	reqPath := srv.URL + "/metadata/instance/compute/userData"
	// Call the getUserData function
	userData, _ := azure.GetUserData(ctx, reqPath)

	// Check that the userData is not empty
	if userData == nil {
		t.Fatalf("getUserData returned empty userData")
	}
}

// TestInvalidGetUserDataInvalidUrl tests the getUserData function with an invalid URL
func TestInvalidGetUserDataInvalidUrl(t *testing.T) {

	// Create a context
	ctx := context.Background()

	// Send request to invalid URL
	reqPath := "invalidURL"
	// Call the getUserData function
	userData, _ := azure.GetUserData(ctx, reqPath)

	// Check that the userData is empty
	if userData != nil {
		t.Fatalf("getUserData returned non-empty userData")
	}
}

// TestInvalidGetUserDataEmptyUrl tests the getUserData function with an empty URL
func TestInvalidGetUserDataEmptyUrl(t *testing.T) {

	// Create a context
	ctx := context.Background()

	// Send request to empty URL
	reqPath := ""
	// Call the getUserData function
	userData, _ := azure.GetUserData(ctx, reqPath)

	// Check that the userData is empty
	if userData != nil {
		t.Fatalf("getUserData returned non-empty userData")
	}
}

type TestProvider struct {
	content  string
	failNext bool
}

func (p *TestProvider) GetUserData(ctx context.Context) ([]byte, error) {
	if p.failNext {
		p.failNext = false
		return []byte("%$#"), nil
	}
	return []byte(p.content), nil
}

func (p *TestProvider) GetRetryDelay() time.Duration {
	return 1 * time.Millisecond
}

// TestRetrieveCloudConfig tests retrieving and parsing of a daemon config
func TestRetrieveCloudConfig(t *testing.T) {
	var provider TestProvider

	provider = TestProvider{content: "write_files: []"}
	_, err := retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve and parse empty cloud config: %v", err)
	}

	provider = TestProvider{failNext: true, content: "write_files: []"}
	_, err = retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}

	provider = TestProvider{content: `#cloud-config
write_files:
- path: /test
  content: |
    test
    test`}
	_, err = retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve valid cloud config: %v", err)
	}
}

func indentTextBlock(text string, by int) string {
	whiteSpace := strings.Repeat(" ", by)
	split := strings.Split(text, "\n")
	indented := ""
	for _, line := range split {
		indented += whiteSpace + line + "\n"
	}
	return indented
}

func TestProcessCloudConfig(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tmp_writefiles_root")
	defer os.RemoveAll(tempDir)

	var aaCfgPath = filepath.Join(tempDir, "aa.toml")
	var agentCfgPath = filepath.Join(tempDir, "agent-config.toml")
	var cdhCfgPath = filepath.Join(tempDir, "cdh.toml")
	var daemonPath = filepath.Join(tempDir, "daemon.json")
	var authPath = filepath.Join(tempDir, "auth.json")
	var initdataPath = filepath.Join(tempDir, "initdata")
	var writeFilesList = []string{aaCfgPath, agentCfgPath, cdhCfgPath, daemonPath, authPath, initdataPath}

	content := fmt.Sprintf(`#cloud-config
write_files:
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
`,
		aaCfgPath,
		indentTextBlock(testAAConfig, 4),
		agentCfgPath,
		indentTextBlock(testAgentConfig, 4),
		cdhCfgPath,
		indentTextBlock(testCDHConfig, 4),
		daemonPath,
		indentTextBlock(testDaemonConfig, 4),
		authPath,
		indentTextBlock(testAuthJson, 4),
		initdataPath,
		indentTextBlock(cc_init_data, 4))

	provider := TestProvider{content: content}

	cc, err := retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve cloud config: %v", err)
	}

	cfg := Config{
		fetchTimeout:  180,
		digestPath:    "",
		initdataPath:  initdataPath,
		parentPath:    tempDir,
		writeFiles:    writeFilesList,
		initdataFiles: nil,
	}
	if err := processCloudConfig(&cfg, cc); err != nil {
		t.Fatalf("failed to process cloud config file: %v", err)
	}

	// check if files have been written correctly
	data, _ := os.ReadFile(aaCfgPath)
	fileContent := string(data)
	if fileContent != testAAConfig {
		t.Fatalf("file content does not match aa config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(agentCfgPath)
	fileContent = string(data)
	if fileContent != testAgentConfig {
		t.Fatalf("file content does not match agent config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(cdhCfgPath)
	fileContent = string(data)
	if fileContent != testCDHConfig {
		t.Fatalf("file content does not match cdh config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(daemonPath)
	fileContent = string(data)
	if fileContent != testDaemonConfig {
		t.Fatalf("file content does not match daemon config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(authPath)
	fileContent = string(data)
	if fileContent != testAuthJson {
		t.Fatalf("file content does not match auth json fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(initdataPath)
	fileContent = string(data)
	if fileContent != cc_init_data+"\n" {
		t.Fatalf("file content does not match initdata fixture: got %q", fileContent)
	}
}

func TestProcessCloudConfigWithMalicious(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tmp_writefiles_root")
	defer os.RemoveAll(tempDir)

	var aaCfgPath = filepath.Join(tempDir, "aa.toml")
	var agentCfgPath = filepath.Join(tempDir, "agent-config.toml")
	var cdhCfgPath = filepath.Join(tempDir, "cdh.toml")
	var daemonPath = filepath.Join(tempDir, "daemon.json")
	var authPath = filepath.Join(tempDir, "auth.json")
	var malicious = filepath.Join(tempDir, "malicious")
	var writeFilesList = []string{aaCfgPath, agentCfgPath, cdhCfgPath, daemonPath, authPath}

	content := fmt.Sprintf(`#cloud-config
write_files:
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
- path: %s
  content: |
%s
`,
		aaCfgPath,
		indentTextBlock(testAAConfig, 4),
		agentCfgPath,
		indentTextBlock(testAgentConfig, 4),
		cdhCfgPath,
		indentTextBlock(testCDHConfig, 4),
		daemonPath,
		indentTextBlock(testDaemonConfig, 4),
		authPath,
		indentTextBlock(testAuthJson, 4),
		malicious,
		indentTextBlock("malicious", 4))

	provider := TestProvider{content: content}

	cc, err := retrieveCloudConfig(context.TODO(), &provider)
	if err != nil {
		t.Fatalf("couldn't retrieve cloud config: %v", err)
	}

	cfg := Config{
		fetchTimeout:  180,
		digestPath:    "",
		initdataPath:  "",
		parentPath:    tempDir,
		writeFiles:    writeFilesList,
		initdataFiles: nil,
	}
	if err := processCloudConfig(&cfg, cc); err != nil {
		t.Fatalf("failed to process cloud config file: %v", err)
	}

	// check if files have been written correctly
	data, _ := os.ReadFile(aaCfgPath)
	fileContent := string(data)
	if fileContent != testAAConfig {
		t.Fatalf("file content does not match aa config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(agentCfgPath)
	fileContent = string(data)
	if fileContent != testAgentConfig {
		t.Fatalf("file content does not match agent config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(cdhCfgPath)
	fileContent = string(data)
	if fileContent != testCDHConfig {
		t.Fatalf("file content does not match cdh config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(daemonPath)
	fileContent = string(data)
	if fileContent != testDaemonConfig {
		t.Fatalf("file content does not match daemon config fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(authPath)
	fileContent = string(data)
	if fileContent != testAuthJson {
		t.Fatalf("file content does not match auth json fixture: got %q", fileContent)
	}

	data, _ = os.ReadFile(malicious)
	if data != nil {
		t.Fatalf("file content should be nil but: got %q", string(data))
	}
}

// TestFailPlainTextUserData tests with plain text userData
func TestFailPlainTextUserData(t *testing.T) {
	// startTestServerPlainText
	srv := startTestServerPlainText()
	defer srv.Close()

	// Create a context
	ctx := context.Background()

	// Send request to srv.URL at path /metadata/instance/compute/userData

	reqPath := srv.URL + "/metadata/instance/compute/userData"
	// Call the getUserData function
	userData, _ := azure.GetUserData(ctx, reqPath)

	// Check that the userData is empty. Since plain text userData is not supported
	if userData != nil {
		t.Fatalf("getUserData returned userData")
	}

}

func TestExtractInitdataAndHash(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tmp_initdata_root")
	defer os.RemoveAll(tempDir)

	var initdataPath = filepath.Join(tempDir, "initdata")
	var aaPath = filepath.Join(tempDir, "aa.toml")
	var cdhPath = filepath.Join(tempDir, "cdh.toml")
	var policyPath = filepath.Join(tempDir, "policy.rego")
	var digestPath = filepath.Join(tempDir, "initdata.digest")
	var initdDataFilesList = []string{aaPath, cdhPath, policyPath}

	cfg := Config{
		fetchTimeout:  180,
		digestPath:    digestPath,
		initdataPath:  initdataPath,
		parentPath:    tempDir,
		writeFiles:    nil,
		initdataFiles: initdDataFilesList,
	}

	_ = writeFile(initdataPath, []byte(cc_init_data))
	err := extractInitdataAndHash(&cfg)
	if err != nil {
		t.Fatalf("extractInitdataAndHash returned err: %v", err)
	}

	bytes, _ := os.ReadFile(aaPath)
	aaStr := string(bytes)
	if testAAConfig != aaStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", aaStr, testAAConfig)
	}

	bytes, _ = os.ReadFile(cdhPath)
	cdhStr := string(bytes)
	if testCDHConfig != cdhStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", cdhStr, testCDHConfig)
	}

	bytes, _ = os.ReadFile(policyPath)
	regoStr := string(bytes)
	if testPolicyConfig != regoStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", regoStr, testPolicyConfig)
	}

	bytes, _ = os.ReadFile(digestPath)
	sum := string(bytes)
	if testCheckSum != sum {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", sum, testCheckSum)
	}
}

func TestWithoutInitdata(t *testing.T) {
	cfg := Config{
		fetchTimeout:  0,
		digestPath:    "",
		initdataPath:  "/does/not/exist",
		parentPath:    "",
		writeFiles:    nil,
		initdataFiles: []string{},
	}

	err := extractInitdataAndHash(&cfg)
	if err != nil {
		t.Fatalf("extractInitdataAndHash returned err: %v", err)
	}
}

func TestExtractInitdataWithMalicious(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tmp_initdata_root")
	defer os.RemoveAll(tempDir)

	var initdataPath = filepath.Join(tempDir, "initdata")
	var aaPath = filepath.Join(tempDir, "aa.toml")
	var cdhPath = filepath.Join(tempDir, "cdh.toml")
	var policyPath = filepath.Join(tempDir, "malicious.rego")
	var digestPath = filepath.Join(tempDir, "initdata.digest")
	var initdDataFilesList = []string{aaPath, cdhPath, policyPath}

	cfg := Config{
		fetchTimeout:  180,
		digestPath:    digestPath,
		initdataPath:  initdataPath,
		parentPath:    tempDir,
		writeFiles:    nil,
		initdataFiles: initdDataFilesList,
	}

	_ = writeFile(initdataPath, []byte(cc_init_data))
	err := extractInitdataAndHash(&cfg)
	if err != nil {
		t.Fatalf("extractInitdataAndHash returned err: %v", err)
	}

	bytes, _ := os.ReadFile(aaPath)
	aaStr := string(bytes)
	if testAAConfig != aaStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", aaStr, testAAConfig)
	}

	bytes, _ = os.ReadFile(cdhPath)
	cdhStr := string(bytes)
	if testCDHConfig != cdhStr {
		t.Fatalf("extractInitdataAndHash returned: %s does not match %s", cdhStr, testCDHConfig)
	}

	bytes, _ = os.ReadFile(policyPath)
	if bytes != nil {
		t.Fatalf("Should not read malicious file but got %s", string(bytes))
	}
}
