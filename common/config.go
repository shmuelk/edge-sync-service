package common

// config contains functions and structs for dealing with the configuration file.

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/open-horizon/edge-utilities/properties"
)

// Cloud Sync Service or Edge Sync Service
const (
	CSS = "CSS"
	ESS = "ESS"
)

// Listening types
const (
	ListeningBoth       = "both"
	ListeningSecurely   = "secure"
	ListeningUnsecurely = "unsecure"
	ListeningUnix       = "unix"
)

// Protocol definitions
const (
	MQTTProtocol = "mqtt"
	HTTPProtocol = "http"
	HybridMQTT   = "hybrid-mqtt"
	HybridWIoTP  = "hybrid-wiotp"
	WIoTP        = "wiotp"
)

// The parallelism modes by which incoming MQTT messages are processed
const (
	ParallelMQTTNone   = "none"
	ParallelMQTTSmall  = "small"
	ParallelMQTTMedium = "medium"
	ParallelMQTTLarge  = "large"
)

// DefaultLogTraceFileSize default value for log and trace file size in KB
const DefaultLogTraceFileSize = 20000

// Config contains the parsed contents of the configuration file
type Config struct {
	// NodeType specifies whether this node is a CSS or ESS
	NodeType string `env:"NODE_TYPE"`

	// DestinationType specifies the destination type of this node
	DestinationType string `env:"DESTINATION_TYPE"`

	// DestinationID specifies the destination id of this node
	DestinationID string `config:"DestinationId" env:"DESTINATION_ID"`

	// OrgID specifies the organization ID of this node
	OrgID string `config:"OrgId" env:"ORG_ID"`

	// ListeningType specifies whether or the server is
	// listening securely, unsecurely, both securely and unsecurely, or using Unix sockets
	// Possible values are secure, unsecure, both, unix.
	// unix can only be specified if the NodeType is ESS.
	// Defaults to unsecure on a CSS and secure on an ESS
	ListeningType string `env:"LISTENING_TYPE"`

	// ListeningAddress specifies the address to listen on
	// If the ListeningType is unix, this property specifies the socket file to be used.
	// The file will be erased and recreated, if it already exists. The filename is relative
	// to the PersistenceRootPath configuration property if it doesn't start with a slash (/).
	ListeningAddress string `env:"LISTENING_ADDRESS"`

	// SecureListeningPort specifies the port to listen on for HTTPS
	SecureListeningPort uint16 `env:"SECURE_LISTENING_PORT"`

	// UnsecureListeningPort specifies the port the CSS listens on for HTTP
	UnsecureListeningPort uint16 `env:"UNSECURE_LISTENING_PORT"`

	// ServerCertificate specifies the Server side certificate to use to serve as HTTPS.
	// This value can either be the certificate itself or the path of a file containing
	// the certificate. If it is a path of a file, then it is relative to the
	// PersistenceRootPath configuration property if it doesn't start with a slash (/).
	ServerCertificate string `env:"SERVER_CERTIFICATE"`

	// ServerKey specifies the Server side key to use to serve as HTTPS.
	// This value can either be the key itself or the path of a file containing the key.
	// If it is a path of a file, then it is relative to the PersistenceRootPath configuration
	// property if it doesn't start with a slash (/).
	ServerKey string `env:"SERVER_KEY"`

	// CSSOnWIoTP indicates whether the CSS is inside or outside the WIoTP.
	// The default value is false, i.e. outside.
	CSSOnWIoTP bool `env:"CSS_ON_WIOTP"`

	// UsingEdgeConnector indicates whether or not the ESS should connect to the Watson IoT Platform via an Edge Connector.
	// Not relevant to the CSS.
	// The default value is false.
	UsingEdgeConnector bool `env:"USING_EDGE_CONNECTOR"`

	// LeadershipTimeout is the timeout for leadership updates in seconds
	LeadershipTimeout int32 `env:"LEADERSHIP_TIMEOUT"`

	// CommunicationProtocol is a comma separated list of protocols to be used for communication between CSS and ESS
	//  The elements of the list can be 'http', 'mqtt', and 'wiotp'
	//  wiotp indicates MQTT communication via the Watson IoT Platform and mqtt indicates direct MQTT communication to a broker
	//  The list must not include both wiotp and mqtt (only one mode of MQTT communication is allowed)
	//  For ESS only a single protocol is allowed
	//  The default is mqtt
	CommunicationProtocol string `env:"COMMUNICATION_PROTOCOL"`

	// MQTTClientID contains the client id
	MQTTClientID string `config:"MQTTClientId" env:"MQTT_CLIENT_ID"`

	// MQTTUserName contains the MQTT user name
	MQTTUserName string `env:"MQTT_USER_NAME"`

	// MQTTPassword contains the MQTT password
	MQTTPassword string `env:"MQTT_PASSWORD"`

	// MQTTUseSSL specifies whether or not to use SSL connection with  the broker
	MQTTUseSSL bool `env:"MQTT_USE_SSL"`

	// MQTTCACertificate specifies the CA certificate that was used to sign the server certificates
	// used by the MQTT broker. This value can either be the CA certificate itself or the path of a file
	// containing the CA certificate. If it is a path of a file, then it is relative to the
	// PersistenceRootPath configuration property if it doesn't start with a slash (/).
	// Default value: broker/ca/ca.cert.pem
	MQTTCACertificate string `env:"MQTT_CA_CERTIFICATE"`

	// MQTTSSLCert specifies the SSL client certificate of the X509 key pair used to communicate with
	// the MQTT broker. This value can either be the certificate itself or the path of a file containing
	// the certificate. If it is a path of a file, then it is relative to the
	// PersistenceRootPath configuration property if it doesn't start with a slash (/).
	MQTTSSLCert string `env:"MQTT_SSL_CERT"`

	// MQTTSSLKey specifies the SSL client key of the X509 key pair used to communicate with the
	// MQTT broker. This value can either be the key itself or the path of a file containing the
	// key. If it is a path of a file, then it is relative to the
	// PersistenceRootPath configuration property if it doesn't start with a slash (/).
	MQTTSSLKey string `env:"MQTT_SSL_KEY"`

	// MQTTAllowInvalidCertificates specifies that the MQTT client will not attempt to validate the server certificates
	// Please only set this for development purposes! It makes using TLS pointless and is never the right answer.
	// Defaults to false
	MQTTAllowInvalidCertificates bool `env:"MQTT_ALLOW_INVALID_CERTIFICATES"`

	// MQTTBrokerConnectTimeout specifies the timeout (in seconds) of attempts to connect to the MQTT broker on startup
	// Default value 300
	MQTTBrokerConnectTimeout int `env:"MQTT_BROKER_CONNECT_TIMEOUT"`

	// MQTTParallelMode specifies the parallelism mode by which incoming MQTT messages are processed
	// Possible values: "none", "small", "medium", "large"
	// Default is "none" (or empty string), i.e., no threading
	MQTTParallelMode string `env:"PARALLEL_MQTT_MODE"`

	// Root path for storing persisted data.
	//  Default value: /var/wiotp-edge/persist
	PersistenceRootPath string `env:"PERSISTENCE_ROOT_PATH"`

	// BrokerAddress specifies the address to connect to for the MQTT broker or
	// a list of server URIs for environments with multiple MQTT brokers
	BrokerAddress string `env:"BROKER_ADDRESS"`

	// BrokerPort specifies the port to connect to for the MQTT broker
	BrokerPort uint16 `env:"BROKER_PORT"`

	// HTTPPollingInterval specifies the frequency in seconds of ESS HTTP polling for updates
	HTTPPollingInterval uint16 `env:"HTTP_POLLING_INTERVAL"`

	// HTTPCSSHost specifies the CSS host for HTTP communication from ESS
	HTTPCSSHost string `env:"HTTP_CSS_HOST"`

	// HTTPCSSPort specifies the CSS host for HTTP communication from ESS
	HTTPCSSPort uint16 `env:"HTTP_CSS_PORT"`

	// HTTPCSSUseSSL specifies whether or not to use SSL connection with the CSS
	HTTPCSSUseSSL bool `env:"HTTP_CSS_USE_SSL"`

	// HTTPCSSCACertificate specifies the CA certificate that was used to sign the server certificate
	// used by the CSS. This value can either be the CA certificate itself or the path of a file containing
	// the CA certificate. If it is a path of a file, then it is relative to the
	// PersistenceRootPath configuration property if it doesn't start with a slash (/).
	// Default value: none
	HTTPCSSCACertificate string `env:"HTTP_CSS_CA_CERTIFICATE"`

	// LogLevel specifies the logging level in string format
	LogLevel string `env:"LOG_LEVEL"`

	// LogRootPath specifies the root path for the log files
	LogRootPath string `env:"LOG_ROOT_PATH"`

	// LogTraceDestination is a comma separated list of destinations for the logging and tracing
	// The elements of the list can be `file`, `stdout`, `syslog`, and 'glog'
	// 'glog' is golang/glog logger
	LogTraceDestination string `env:"LOG_TRACE_DESTINATION"`

	// LogFileName specifies the name of the log file
	LogFileName string `env:"LOG_FILE_NAME"`

	// TraceLevel specifies the tracing level in string form
	TraceLevel string `env:"TRACE_LEVEL"`

	// TraceRootPath specifies the root path for the trace files
	TraceRootPath string `env:"TRACE_ROOT_PATH"`

	// TraceFileName specifies the name of the trace file
	TraceFileName string `env:"TRACE_FILE_NAME"`

	// Maximal size of a trace/log file in kilo bytes.
	LogTraceFileSizeKB int `env:"LOG_TRACE_FILE_SIZE_KB"`

	// The limit on the number of compressed files of trace/log.
	MaxCompressedlLogTraceFilesNumber int `env:"MAX_COMPRESSED_LOG_TRACE_FILES_NUMBER"`

	// LogTraceMaintenanceInterval specifies the frequency in seconds of log and trace maintenance (memory consumption, etc.)
	LogTraceMaintenanceInterval int16 `env:"LOG_TRACE_MAINTENANCE_INTERVAL"`

	// ResendInterval specifies the frequency in seconds of checks to resend unacknowledged notifications
	// ESS resends register notification with this interval
	// Other notifications are resent with frequency equal to ResendInterval*6
	ResendInterval int16 `env:"RESEND_INTERVAL"`

	// Maximum size of data that can be sent in one message
	MaxDataChunkSize int `env:"MAX_DATA_CHUNK_SIZE"`

	// Max num of inflight chunks
	MaxInflightChunks int `env:"MAX_INFLIGHT_CHUNKS"`

	// MongoAddressCsv specifies one or more addresses of the mongo database
	MongoAddressCsv string `env:"MONGO_ADDRESS_CSV"`

	// MongoAuthDbName specifies the name of the database used to establish credentials and privileges
	MongoAuthDbName string `env:"MONGO_AUTH_DB_NAME"`

	// MongoDbName specifies the name of the database to use
	MongoDbName string `env:"MONGO_DB_NAME"`

	// MongoUsername specifies the username of the mongo database
	MongoUsername string `env:"MONGO_USERNAME"`

	// MongoPassword specifies the username of the mongo database
	MongoPassword string `env:"MONGO_PASSWORD"`

	// MongoUseSSL specifies whether or not to use SSL connection with mongo
	MongoUseSSL bool `env:"MONGO_USE_SSL"`

	// MongoCACertificate specifies the CA certificate that was used to sign the server certificate
	// used by the MongoDB server. This value can either be the CA certificate itself or the path of a
	// file containing the CA certificate. If it is a path of a file, then it is relative to the
	// PersistenceRootPath configuration property if it doesn't start with a slash (/).
	MongoCACertificate string `env:"MONGO_CA_CERTIFICATE"`

	// MongoAllowInvalidCertificates specifies that the mongo driver will not attempt to validate the server certificates.
	// Please only set this for development purposes! It makes using TLS pointless and is never the right answer.
	MongoAllowInvalidCertificates bool `env:"MONGO_ALLOW_INVALID_CERTIFICATES"`

	// MongoSessionCacheSize specifies the number of MongoDB session copies to use
	MongoSessionCacheSize int `env:"MONGO_SESSION_CACHE_SIZE"`

	// DatabaseConnectTimeout specifies that the timeout in seconds of database connection attempts on startup
	// The default value is 300
	DatabaseConnectTimeout int `env:"DATABASE_CONNECT_TIMEOUT"`

	// StorageMaintenanceInterval specifies the frequency in seconds of storage checks (for expired objects, etc.)
	StorageMaintenanceInterval int16 `env:"STORAGE_MAINTENANCE_INTERVAL"`

	// ObjectActivationInterval specifies the frequency in seconds of checking if there are inactive objects
	// that are ready to be activated
	ObjectActivationInterval int16 `env:"OBJECT_ACTIVATION_INTERVAL"`

	// ESSPersistentStorage specifies whether to use a persistent storage (boltdb) or in memory storage.
	// The default value is false, i.e. in memory storage.
	ESSPersistentStorage bool `env:"ESS_PERSISTENT_STORAGE"`

	// MessagingGroupCacheExpiration specifies the expiration time in minutes of organization to messaging group mapping cache
	MessagingGroupCacheExpiration int16 `env:"MESSAGING_GROUP_CACHE_EXPIRATION"`
}

// Configuration contains the read in configuration
var Configuration Config

type configError struct {
	message string
}

func (e *configError) Error() string {
	return e.message
}

// Load loads the configuration from the specified properties file
func Load(configFileName string) error {
	props, err := properties.ReadPropertiesFile(configFileName, true)
	if err != nil {
		return err
	}
	if err = properties.LoadProperties(props, &Configuration, "config"); err != nil {
		return err
	}
	err = properties.LoadEnvironment(&Configuration, "env")
	if err != nil {
		return err
	}

	var protocol string
	if Configuration.HTTPCSSUseSSL {
		protocol = "https"
	} else {
		protocol = "http"
	}
	HTTPCSSURL = fmt.Sprintf("%s://%s:%d", protocol, Configuration.HTTPCSSHost, Configuration.HTTPCSSPort)

	return nil
}

// ValidateConfig Validates the configuration
func ValidateConfig() error {
	if strings.HasPrefix(Configuration.PersistenceRootPath, "./") ||
		strings.HasPrefix(Configuration.PersistenceRootPath, "../") {
		cwd, err := os.Getwd()
		if err != nil {
			return &configError{fmt.Sprintf("Couldn't determine the current directory. Error: %s", err)}
		}
		if strings.HasPrefix(Configuration.PersistenceRootPath, "./") {
			Configuration.PersistenceRootPath = cwd + Configuration.PersistenceRootPath[1:]
		} else {
			Configuration.PersistenceRootPath = cwd + "/" + Configuration.PersistenceRootPath
		}

	}

	if strings.EqualFold(Configuration.NodeType, CSS) {
		Configuration.NodeType = CSS
	} else if strings.EqualFold(Configuration.NodeType, ESS) {
		Configuration.NodeType = ESS
	} else {
		return &configError{fmt.Sprintf("The node type specified in the configuration file (%s) is incorrect. It must be CSS or ESS.",
			Configuration.NodeType)}
	}

	if Configuration.DestinationType == "" {
		return &configError{"Please specify the destination type in the configuration file"}
	}

	if Configuration.DestinationID == "" {
		return &configError{"Please specify the destination id in the configuration file"}
	}

	if Configuration.NodeType == ESS && Configuration.OrgID == "" {
		return &configError{"Please specify the organization id in the configuration file"}
	}

	if Configuration.NodeType == CSS && !Configuration.CSSOnWIoTP && Configuration.OrgID != "" {
		SingleOrgCSS = true
	}

	Configuration.ListeningType = strings.ToLower(Configuration.ListeningType)
	if len(Configuration.ListeningType) == 0 {
		if Configuration.NodeType == ESS {
			Configuration.ListeningType = ListeningSecurely
		} else {
			Configuration.ListeningType = ListeningUnsecurely
		}
	} else if Configuration.ListeningType != ListeningBoth &&
		Configuration.ListeningType != ListeningSecurely && Configuration.ListeningType != ListeningUnsecurely &&
		Configuration.ListeningType != ListeningUnix {
		return &configError{fmt.Sprintf("ListeningType must be %s, %s, %s, or %s", ListeningBoth, ListeningSecurely, ListeningUnsecurely, ListeningUnix)}
	}

	if (Configuration.ListeningType == ListeningUnsecurely || Configuration.ListeningType == ListeningBoth) &&
		Configuration.UnsecureListeningPort == 0 {
		return &configError{"Have requested unsecure API serving, but the UnsecureListeningPort is zero."}
	}
	if (Configuration.ListeningType == ListeningSecurely || Configuration.ListeningType == ListeningBoth) &&
		Configuration.SecureListeningPort == 0 {
		return &configError{"Have requested secure API serving, but the SecureListeningPort is zero."}
	}
	if Configuration.NodeType == CSS {
		if (Configuration.ListeningType == ListeningSecurely || Configuration.ListeningType == ListeningBoth) &&
			len(Configuration.ServerCertificate) == 0 {
			return &configError{"Have requested secure API serving, but no server certificate has been specified."}
		}
		if (Configuration.ListeningType == ListeningSecurely || Configuration.ListeningType == ListeningBoth) &&
			len(Configuration.ServerKey) == 0 {
			return &configError{"Have requested secure API serving, but no server private key has been specified."}
		}
	}
	if Configuration.ListeningType == ListeningUnix {
		if Configuration.NodeType != ESS {
			return &configError{"Only an ESS can listen via Unix Sockets"}
		}
		if len(Configuration.ListeningAddress) == 0 {
			return &configError{"When Listening via Unix Sockets, you must specify the Socket file using the ListeningAddress property"}
		}
	}

	protocols := strings.Split(Configuration.CommunicationProtocol, ",")
	var mqtt, http, wiotp bool
	if len(protocols) == 0 {
		mqtt = true
	} else {
		for _, protocol := range protocols {
			if strings.EqualFold(protocol, "mqtt") {
				mqtt = true
			} else if strings.EqualFold(protocol, "wiotp") {
				wiotp = true
			} else if strings.EqualFold(protocol, "http") {
				http = true
			}
		}
	}

	if !mqtt && !http && !wiotp {
		return &configError{"Invalid communication protocol, please choose either HTTP or MQTT or WIoTP"}
	}

	if Configuration.NodeType == ESS {
		if (mqtt && http) || (mqtt && wiotp) || (http && wiotp) {
			return &configError{"Invalid communication protocol, please choose one of HTTP, MQTT or WIoTP"}
		}
		if mqtt {
			Configuration.CommunicationProtocol = MQTTProtocol
		} else if wiotp {
			Configuration.CommunicationProtocol = WIoTP
		} else {
			Configuration.CommunicationProtocol = HTTPProtocol
		}
	} else {
		if wiotp && mqtt {
			return &configError{"Invalid communication protocol, please choose either MQTT or WIoTP"}
		}
		if http {
			if mqtt {
				Configuration.CommunicationProtocol = HybridMQTT
			} else if wiotp {
				Configuration.CommunicationProtocol = HybridWIoTP
			} else {
				Configuration.CommunicationProtocol = HTTPProtocol
			}
		} else {
			if mqtt {
				Configuration.CommunicationProtocol = MQTTProtocol
			} else if wiotp {
				Configuration.CommunicationProtocol = WIoTP
			}
		}
	}

	if Configuration.NodeType == CSS && Configuration.CSSOnWIoTP && mqtt {
		return &configError{"CSS on Watson IoTP should use wiotp protocol"}
	}

	if (mqtt || (wiotp && (Configuration.UsingEdgeConnector || Configuration.NodeType == CSS))) &&
		Configuration.MQTTUserName == "" && Configuration.MQTTPassword != "" {
		// For ESS connecting not via EC with wiotp we set user name to use-auth-token,
		// otherwise if the password is set, the use name should be set as well
		return &configError{"Please specify the user name for MQTT communication in the configuration file"}
	}

	if Configuration.NodeType == CSS && Configuration.CommunicationProtocol != HTTPProtocol {
		// MQTT and CSS
		if Configuration.CSSOnWIoTP && !strings.HasPrefix(Configuration.BrokerAddress, "[") {
			return &configError{"Please specify the broker addresses for messaging groups"}
		}
		if !Configuration.CSSOnWIoTP && strings.HasPrefix(Configuration.BrokerAddress, "[") {
			return &configError{"Please provide one broker address for CSS outside WIoTP"}
		}
	}

	if Configuration.NodeType == ESS && Configuration.CommunicationProtocol != HTTPProtocol {
		// MQTT and ESS
		if strings.HasPrefix(Configuration.BrokerAddress, "[") {
			return &configError{"Please provide one broker address"}
		}

		if wiotp {
			if Configuration.UsingEdgeConnector {
				if Configuration.MQTTClientID == "" {
					Configuration.MQTTClientID = "a:sync-service"
				}
				if Configuration.BrokerAddress == "" {
					Configuration.BrokerAddress = "edge-connector"
				}
			} else {
				if Configuration.MQTTPassword == "" {
					return &configError{"Please provide password for MQTT connection"}
				}
				Configuration.MQTTUserName = "use-token-auth"

				if Configuration.MQTTClientID == "" {
					Configuration.MQTTClientID = "g:" + Configuration.OrgID + ":" + Configuration.DestinationType +
						":" + Configuration.DestinationID
				}
				if Configuration.BrokerAddress == "" {
					Configuration.BrokerAddress = Configuration.OrgID + ".messaging.internetofthings.ibmcloud.com"
				}
			}
			if Configuration.BrokerPort == 0 {
				Configuration.BrokerPort = 8883
			}
		} else {
			// mqtt
			if Configuration.MQTTClientID == "" {
				Configuration.MQTTClientID = "ESS-" + strconv.FormatInt(rand.Int63n(math.MaxInt64), 10)
			}
			if Configuration.BrokerAddress == "" {
				Configuration.BrokerAddress = "localhost"
			}
			if Configuration.BrokerPort == 0 {
				Configuration.BrokerPort = 1883
			}
		}
	}

	if Configuration.NodeType == CSS && Configuration.CommunicationProtocol != HTTPProtocol {
		// MQTT and CSS
		if wiotp && !Configuration.CSSOnWIoTP {
			if Configuration.BrokerAddress == "" {
				if SingleOrgCSS {
					Configuration.BrokerAddress = Configuration.OrgID + ".messaging.internetofthings.ibmcloud.com"
				} else {
					Configuration.BrokerAddress = "messaging.internetofthings.ibmcloud.com"
				}
			}
			if Configuration.BrokerPort == 0 {
				Configuration.BrokerPort = 8883
			}
			if SingleOrgCSS && Configuration.MQTTClientID == "" {
				Configuration.MQTTClientID = "A:" + Configuration.OrgID + ":CSS"
			}
		} else if mqtt {
			if Configuration.BrokerAddress == "" {
				Configuration.BrokerAddress = "localhost"
			}
			if Configuration.BrokerPort == 0 {
				Configuration.BrokerPort = 1883
			}
			if Configuration.MQTTClientID == "" {
				Configuration.MQTTClientID = "CSS-" + strconv.FormatInt(rand.Int63n(math.MaxInt64), 10)
			}
		}
	}

	if Configuration.NodeType == ESS && Configuration.CommunicationProtocol == HTTPProtocol &&
		(Configuration.HTTPCSSHost == "" || Configuration.HTTPCSSPort == 0) {
		return &configError{"Please specify the host and port of CSS for HTTP communication in the configuration file"}
	}

	if !strings.HasSuffix(Configuration.PersistenceRootPath, "/") {
		Configuration.PersistenceRootPath += "/"
	}

	Configuration.MQTTParallelMode = strings.ToLower(Configuration.MQTTParallelMode)
	switch Configuration.MQTTParallelMode {
	case ParallelMQTTNone:
	case ParallelMQTTSmall:
	case ParallelMQTTMedium:
	case ParallelMQTTLarge:
	case "":
	default:
		return &configError{"Invalid MQTTParallelMode, please specify any off: 'none', 'small', 'medium', 'large', or leave as empty string"}
	}

	if Configuration.MaxInflightChunks < 1 {
		Configuration.MaxInflightChunks = 1
	}
	if Configuration.MaxInflightChunks > 64 && Configuration.NodeType == CSS {
		Configuration.MaxInflightChunks = 64
	}

	return nil
}

func init() {
	Configuration.NodeType = CSS
	Configuration.ListeningType = ""
	Configuration.ListeningAddress = ""
	Configuration.SecureListeningPort = 8443
	Configuration.UnsecureListeningPort = 8080
	Configuration.LeadershipTimeout = 30
	Configuration.CSSOnWIoTP = false
	Configuration.UsingEdgeConnector = false
	Configuration.MQTTUseSSL = true
	Configuration.MQTTAllowInvalidCertificates = false
	Configuration.PersistenceRootPath = "/var/wiotp-edge/persist"
	Configuration.MQTTCACertificate = "broker/ca/ca.cert.pem"
	Configuration.MQTTBrokerConnectTimeout = 300
	Configuration.LogLevel = "INFO"
	Configuration.LogRootPath = "/var/edge-sync-service/log"
	Configuration.LogFileName = "sync-service"
	Configuration.TraceLevel = "INFO"
	Configuration.TraceRootPath = "/var/edge-sync-service/trace"
	Configuration.TraceFileName = "sync-service"
	Configuration.LogTraceFileSizeKB = DefaultLogTraceFileSize
	Configuration.MaxCompressedlLogTraceFilesNumber = 50
	Configuration.LogTraceDestination = "file"
	Configuration.LogTraceMaintenanceInterval = 60
	Configuration.ResendInterval = 5
	Configuration.MaxDataChunkSize = 120 * 1024
	Configuration.MaxInflightChunks = 1
	Configuration.MongoAddressCsv = "localhost:27017"
	Configuration.MongoDbName = "d_edge"
	Configuration.MongoAuthDbName = "admin"
	Configuration.MongoUsername = ""
	Configuration.MongoPassword = ""
	Configuration.MongoUseSSL = false
	Configuration.MongoCACertificate = ""
	Configuration.MongoAllowInvalidCertificates = false
	Configuration.MongoSessionCacheSize = 1
	Configuration.DatabaseConnectTimeout = 300
	Configuration.StorageMaintenanceInterval = 30
	Configuration.ObjectActivationInterval = 30
	Configuration.CommunicationProtocol = MQTTProtocol
	Configuration.HTTPPollingInterval = 10
	Configuration.HTTPCSSUseSSL = false
	Configuration.HTTPCSSCACertificate = ""
	Configuration.MessagingGroupCacheExpiration = 60
}