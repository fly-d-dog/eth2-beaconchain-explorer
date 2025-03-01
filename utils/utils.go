package utils

import (
	"bytes"
	securerand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"eth2-exporter/price"
	"eth2-exporter/types"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v2"

	"github.com/kataras/i18n"
	"github.com/kelseyhightower/envconfig"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

// Config is the globally accessible configuration
var Config *types.Config

var localiser *i18n.I18n

// making sure language files are loaded only once
func getLocaliser() *i18n.I18n {
	if localiser == nil {
		localiser, err := i18n.New(i18n.Glob("locales/*/*"), "en-US", "ru-RU")
		if err != nil {
			log.Println(err)
		}
		return localiser
	}
	return localiser
}

// GetTemplateFuncs will get the template functions
func GetTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"includeHTML":                             IncludeHTML,
		"formatHTML":                              FormatMessageToHtml,
		"formatBalance":                           FormatBalance,
		"formatBalanceSql":                        FormatBalanceSql,
		"formatCurrentBalance":                    FormatCurrentBalance,
		"formatEffectiveBalance":                  FormatEffectiveBalance,
		"formatBlockStatus":                       FormatBlockStatus,
		"formatBlockSlot":                         FormatBlockSlot,
		"formatSlotToTimestamp":                   FormatSlotToTimestamp,
		"formatDepositAmount":                     FormatDepositAmount,
		"formatEpoch":                             FormatEpoch,
		"formatEth1Block":                         FormatEth1Block,
		"formatEth1Address":                       FormatEth1Address,
		"formatEth1TxHash":                        FormatEth1TxHash,
		"formatGraffiti":                          FormatGraffiti,
		"formatHash":                              FormatHash,
		"formatBitlist":                           FormatBitlist,
		"formatBitvectorValidators":               formatBitvectorValidators,
		"formatParticipation":                     FormatParticipation,
		"formatIncome":                            FormatIncome,
		"formatMoney":                             FormatMoney,
		"formatIncomeSql":                         FormatIncomeSql,
		"formatSqlInt64":                          FormatSqlInt64,
		"formatValidator":                         FormatValidator,
		"formatValidatorWithName":                 FormatValidatorWithName,
		"formatValidatorInt64":                    FormatValidatorInt64,
		"formatValidatorStatus":                   FormatValidatorStatus,
		"formatPercentage":                        FormatPercentage,
		"formatPercentageWithPrecision":           FormatPercentageWithPrecision,
		"formatPercentageWithGPrecision":          FormatPercentageWithGPrecision,
		"formatPercentageColored":                 FormatPercentageColored,
		"formatPercentageColoredEmoji":            FormatPercentageColoredEmoji,
		"formatPublicKey":                         FormatPublicKey,
		"formatSlashedValidator":                  FormatSlashedValidator,
		"formatSlashedValidatorInt64":             FormatSlashedValidatorInt64,
		"formatTimestamp":                         FormatTimestamp,
		"formatTsWithoutTooltip":                  FormatTsWithoutTooltip,
		"formatTimestampTs":                       FormatTimestampTs,
		"formatValidatorName":                     FormatValidatorName,
		"formatAttestationInclusionEffectiveness": FormatAttestationInclusionEffectiveness,
		"formatValidatorTags":                     FormatValidatorTags,
		"formatValidatorTag":                      FormatValidatorTag,
		"formatRPL":                               FormatRPL,
		"formatFloatWithPrecision":                FormatFloatWithPrecision,
		"epochOfSlot":                             EpochOfSlot,
		"dayToTime":                               DayToTime,
		"contains":                                strings.Contains,
		"roundDecimals":                           RoundDecimals,
		"mod":                                     func(i, j int) bool { return i%j == 0 },
		"sub":                                     func(i, j int) int { return i - j },
		"add":                                     func(i, j int) int { return i + j },
		"addI64":                                  func(i, j int64) int64 { return i + j },
		"mul":                                     func(i, j float64) float64 { return i * j },
		"div":                                     func(i, j float64) float64 { return i / j },
		"divInt":                                  func(i, j int) float64 { return float64(i) / float64(j) },
		"gtf":                                     func(i, j float64) bool { return i > j },
		"round": func(i float64, n int) float64 {
			return math.Round(i*math.Pow10(n)) / math.Pow10(n)
		},
		"percent": func(i float64) float64 { return i * 100 },
		"formatThousands": func(i float64) string {
			p := message.NewPrinter(language.English)
			return p.Sprintf("%.0f\n", i)
		},
		"formatThousandsInt": func(i int) string {
			p := message.NewPrinter(language.English)
			return p.Sprintf("%d", i)
		},
		"derefString":      DerefString,
		"trLang":           TrLang,
		"firstCharToUpper": func(s string) string { return strings.Title(s) },
		"eqsp": func(a, b *string) bool {
			if a != nil && b != nil {
				return *a == *b
			}
			return false
		},
		"stringsJoin":     strings.Join,
		"formatAddCommas": FormatAddCommas,
	}
}

var LayoutPaths []string = []string{"templates/layout/layout.html", "templates/layout/nav.html"}

// IncludeHTML adds html to the page
func IncludeHTML(path string) template.HTML {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("includeHTML - error reading file: %v", err)
		return ""
	}
	return template.HTML(string(b))
}

func GraffitiToSring(graffiti []byte) string {
	return strings.Map(fixUtf, string(bytes.Trim(graffiti, "\x00")))
}

// FormatGraffitiString formats (and escapes) the graffiti
func FormatGraffitiString(graffiti string) string {
	return strings.Map(fixUtf, template.HTMLEscapeString(graffiti))
}

func fixUtf(r rune) rune {
	if r == utf8.RuneError {
		return -1
	}
	return r
}

func SyncPeriodOfEpoch(epoch uint64) uint64 {
	if epoch < Config.Chain.AltairForkEpoch {
		return 0
	}
	return epoch / Config.Chain.EpochsPerSyncCommitteePeriod
}

func FirstEpochOfSyncPeriod(syncPeriod uint64) uint64 {
	return syncPeriod * Config.Chain.EpochsPerSyncCommitteePeriod
}

func TimeToSyncPeriod(t time.Time) uint64 {
	return SyncPeriodOfEpoch(uint64(TimeToEpoch(t)))
}

// EpochOfSlot returns the corresponding epoch of a slot
func EpochOfSlot(slot uint64) uint64 {
	return slot / Config.Chain.SlotsPerEpoch
}

// DayOfSlot returns the corresponding day of a slot
func DayOfSlot(slot uint64) uint64 {
	return Config.Chain.SecondsPerSlot * slot / (24 * 3600)
}

// WeekOfSlot returns the corresponding week of a slot
func WeekOfSlot(slot uint64) uint64 {
	return Config.Chain.SecondsPerSlot * slot / (7 * 24 * 3600)
}

// SlotToTime returns a time.Time to slot
func SlotToTime(slot uint64) time.Time {
	return time.Unix(int64(Config.Chain.GenesisTimestamp+slot*Config.Chain.SecondsPerSlot), 0)
}

// TimeToSlot returns time to slot in seconds
func TimeToSlot(timestamp uint64) uint64 {
	if Config.Chain.GenesisTimestamp > timestamp {
		return 0
	}
	return (timestamp - Config.Chain.GenesisTimestamp) / Config.Chain.SecondsPerSlot
}

// EpochToTime will return a time.Time for an epoch
func EpochToTime(epoch uint64) time.Time {
	return time.Unix(int64(Config.Chain.GenesisTimestamp+epoch*Config.Chain.SecondsPerSlot*Config.Chain.SlotsPerEpoch), 0)
}

// TimeToDay will return a days since genesis for an timestamp
func TimeToDay(timestamp uint64) uint64 {
	return uint64(time.Unix(int64(timestamp), 0).Sub(time.Unix(int64(Config.Chain.GenesisTimestamp), 0)).Hours() / 24)
	// return time.Unix(int64(Config.Chain.GenesisTimestamp), 0).Add(time.Hour * time.Duration(24*int(day)))
}

func DayToTime(day int64) time.Time {
	return time.Unix(int64(Config.Chain.GenesisTimestamp), 0).Add(time.Hour * time.Duration(24*int(day)))
}

// TimeToEpoch will return an epoch for a given time
func TimeToEpoch(ts time.Time) int64 {
	if int64(Config.Chain.GenesisTimestamp) > ts.Unix() {
		return 0
	}
	return (ts.Unix() - int64(Config.Chain.GenesisTimestamp)) / int64(Config.Chain.SecondsPerSlot) / int64(Config.Chain.SlotsPerEpoch)
}

// WaitForCtrlC will block/wait until a control-c is pressed
func WaitForCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

// ReadConfig will process a configuration
func ReadConfig(cfg *types.Config, path string) error {
	err := readConfigFile(cfg, path)

	if err != nil {
		return err
	}

	readConfigEnv(cfg)
	err = readConfigSecrets(cfg)
	if err != nil {
		return err
	}

	// decode phase0 config
	if len(cfg.Chain.Phase0Path) == 0 {
		cfg.Chain.Phase0Path = "config/phase0.yml"
	}
	phase0 := &types.Phase0{}
	f, err := os.Open(cfg.Chain.Phase0Path)
	if err != nil {
		logrus.Errorf("error opening Phase0 Config file %v: %v", cfg.Chain.Phase0Path, err)
	} else {
		decoder := yaml.NewDecoder(f)
		err = decoder.Decode(phase0)
		if err != nil {
			logrus.Errorf("error decoding Phase0 Config file %v: %v", cfg.Chain.Phase0Path, err)
		} else {
			cfg.Chain.Phase0 = *phase0
		}
	}

	// decode altair config
	if len(cfg.Chain.AltairPath) == 0 {
		cfg.Chain.AltairPath = "config/altair.yml"
	}
	altair := &types.Altair{}
	f, err = os.Open(cfg.Chain.AltairPath)
	if err != nil {
		logrus.Errorf("error opening altair config file %v: %v", cfg.Chain.AltairPath, err)
	} else {
		decoder := yaml.NewDecoder(f)
		err = decoder.Decode(altair)
		if err != nil {
			logrus.Errorf("error decoding altair Config file %v: %v", cfg.Chain.AltairPath, err)
		} else {
			cfg.Chain.Altair = *altair
		}
	}

	return nil
}

func readConfigFile(cfg *types.Config, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening config file %v: %v", path, err)
	}

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		return fmt.Errorf("error decoding config file %v: %v", path, err)
	}

	return nil
}

func readConfigEnv(cfg *types.Config) error {
	return envconfig.Process("", cfg)
}

func readConfigSecrets(cfg *types.Config) error {
	return ProcessSecrets(cfg)
}

// MustParseHex will parse a string into hex
func MustParseHex(hexString string) []byte {
	data, err := hex.DecodeString(strings.Replace(hexString, "0x", "", -1))
	if err != nil {
		log.Fatal(err)
	}
	return data
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Headers", "*, Authorization")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
		return
	})
}

func IsApiRequest(r *http.Request) bool {
	query, ok := r.URL.Query()["format"]
	return ok && len(query) > 0 && query[0] == "json"
}

var eth1AddressRE = regexp.MustCompile("^0?x?[0-9a-fA-F]{40}$")
var zeroHashRE = regexp.MustCompile("^0?x?0+$")

// IsValidEth1Address verifies whether a string represents a valid eth1-address.
func IsValidEth1Address(s string) bool {
	return !zeroHashRE.MatchString(s) && eth1AddressRE.MatchString(s)
}

// https://github.com/badoux/checkmail/blob/f9f80cb795fa/checkmail.go#L37
var emailRE = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// IsValidEmail verifies wheter a string represents a valid email-address.
func IsValidEmail(s string) bool {
	return emailRE.MatchString(s)
}

// RoundDecimals rounds (nearest) a number to the specified number of digits after comma
func RoundDecimals(f float64, n int) float64 {
	d := math.Pow10(n)
	return math.Round(f*d) / d
}

// HashAndEncode digests the input with sha256 and returns it as hex string
func HashAndEncode(input string) string {
	codeHashedBytes := sha256.Sum256([]byte(input))
	return hex.EncodeToString(codeHashedBytes[:])
}

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

// RandomString returns a random hex-string
func RandomString(length int) string {
	b, _ := GenerateRandomBytesSecure(length)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

func GenerateRandomBytesSecure(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := securerand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func SqlRowsToJSON(rows *sql.Rows) ([]interface{}, error) {
	columnTypes, err := rows.ColumnTypes()

	if err != nil {
		return nil, err
	}

	count := len(columnTypes)
	finalRows := []interface{}{}

	for rows.Next() {

		scanArgs := make([]interface{}, count)

		for i, v := range columnTypes {
			switch v.DatabaseTypeName() {
			case "VARCHAR", "TEXT", "UUID":
				scanArgs[i] = new(sql.NullString)
				break
			case "BOOL":
				scanArgs[i] = new(sql.NullBool)
				break
			case "INT4", "INT8":
				scanArgs[i] = new(sql.NullInt64)
				break
			case "FLOAT8":
				scanArgs[i] = new(sql.NullFloat64)
				break
			case "TIMESTAMP":
				scanArgs[i] = new(sql.NullTime)
				break
			case "_INT4", "_INT8":
				scanArgs[i] = new(pq.Int64Array)
				break
			default:
				scanArgs[i] = new(sql.NullString)
			}
		}

		err := rows.Scan(scanArgs...)

		if err != nil {
			return nil, err
		}

		masterData := map[string]interface{}{}

		for i, v := range columnTypes {

			//log.Println(v.Name(), v.DatabaseTypeName())
			if z, ok := (scanArgs[i]).(*sql.NullBool); ok {
				if z.Valid {
					masterData[v.Name()] = z.Bool
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullString); ok {
				if z.Valid {
					if v.DatabaseTypeName() == "BYTEA" {
						if len(z.String) > 0 {
							masterData[v.Name()] = "0x" + hex.EncodeToString([]byte(z.String))
						} else {
							masterData[v.Name()] = nil
						}
					} else if v.DatabaseTypeName() == "NUMERIC" {
						nbr, _ := new(big.Int).SetString(z.String, 10)
						masterData[v.Name()] = nbr
					} else {
						masterData[v.Name()] = z.String
					}
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt64); ok {
				if z.Valid {
					masterData[v.Name()] = z.Int64
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt32); ok {
				if z.Valid {
					masterData[v.Name()] = z.Int32
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullFloat64); ok {
				if z.Valid {
					masterData[v.Name()] = z.Float64
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullTime); ok {
				if z.Valid {
					masterData[v.Name()] = z.Time.Unix()
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			masterData[v.Name()] = scanArgs[i]
		}

		finalRows = append(finalRows, masterData)
	}

	return finalRows, nil
}

// GenerateAPIKey generates an API key for a user
func GenerateAPIKey(passwordHash, email, Ts string) (string, error) {
	apiKey, err := bcrypt.GenerateFromPassword([]byte(passwordHash+email+Ts), 10)
	if err != nil {
		return "", err
	}
	key := apiKey
	if len(apiKey) > 30 {
		key = apiKey[8:29]
	}

	apiKeyBase64 := base64.RawURLEncoding.EncodeToString(key)
	return apiKeyBase64, nil
}

func ExchangeRateForCurrency(currency string) float64 {
	return price.GetEthPrice(currency)
}

// Glob walks through a directory and returns files with a given extention
func Glob(dir string, ext string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ext {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// ValidateReCAPTCHA validates a ReCaptcha server side
func ValidateReCAPTCHA(recaptchaResponse string) (bool, error) {
	// Check this URL verification details from Google
	// https://developers.google.com/recaptcha/docs/verify
	req, err := http.PostForm("https://www.google.com/recaptcha/api/siteverify", url.Values{
		"secret":   {Config.Frontend.RecaptchaSecretKey},
		"response": {recaptchaResponse},
	})
	if err != nil { // Handle error from HTTP POST to Google reCAPTCHA verify server
		return false, err
	}
	defer req.Body.Close()
	body, err := ioutil.ReadAll(req.Body) // Read the response from Google
	if err != nil {
		return false, err
	}

	var googleResponse types.GoogleRecaptchaResponse
	err = json.Unmarshal(body, &googleResponse) // Parse the JSON response from Google
	if err != nil {
		return false, err
	}
	if len(googleResponse.ErrorCodes) > 0 {
		err = fmt.Errorf("Error validating ReCaptcha %v", googleResponse.ErrorCodes)
	} else {
		err = nil
	}

	if googleResponse.Score > 0.5 {
		return true, err
	}

	return false, fmt.Errorf("Score too low threshold not reached, Score: %v - Required >0.5; %v", googleResponse.Score, err)
}

func BitAtVector(b []byte, i int) bool {
	bb := b[i/8]
	return (bb & (1 << uint(i%8))) > 0
}

func BitAtVectorReversed(b []byte, i int) bool {
	bb := b[i/8]
	return (bb & (1 << uint(7-(i%8)))) > 0
}

func GetNetwork() string {
	if Config.Chain.Network != "" {
		return strings.ToLower(Config.Chain.Network)
	}
	return strings.ToLower(Config.Chain.Phase0.ConfigName)
}
