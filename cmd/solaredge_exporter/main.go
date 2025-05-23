package main

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/goburrow/modbus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"paepcke.de/solaredge_exporter/config"
	"paepcke.de/solaredge_exporter/exporter"
	"paepcke.de/solaredge_exporter/solaredge"
)

func main() {
	// configuration
	config.InitConfig()

	// logger
	m := zerolog.ConsoleWriter{Out: os.Stdout}
	log.Logger = log.Output(zerolog.SyncWriter(m))
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if viper.GetBool("Log.Debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	log.Info().Msg("Starting solaredge_exporter")
	log.Info().Msgf("Version: %s", config.VersionInfo)
	log.Info().Msgf("Configured Inverter Address: %s", viper.GetString("SolarEdge.InverterAddress"))
	log.Info().Msgf("Configured Inverter Port: %d", viper.GetInt("SolarEdge.InverterPort"))
	log.Info().Msgf("Configured Listen Address: %s", viper.GetString("Exporter.ListenAddress"))
	log.Info().Msgf("Configured Listen Port: %d", viper.GetInt("Exporter.ListenPort"))

	// Start Data Collection
	// TODO: Add a cancellation context on SIGINT to cleanly close the connection
	go runCollection()

	// Start Prometheus Handler
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(viper.GetString("Exporter.ListenAddress")+":"+strconv.Itoa(viper.GetInt("Exporter.ListenPort")), nil)
	if err != nil {
		log.Error().Msgf("Could not start the prometheus metric server: %s", err.Error())
	}
}

func runCollection() {
	// Get Interval from Config
	interval := viper.GetInt("Exporter.Interval")

	// Configure Modbus Connection and Handler/Client
	handler := modbus.NewTCPClientHandler(
		fmt.Sprintf("%s:%d",
			viper.GetString("SolarEdge.InverterAddress"),
			viper.GetInt("SolarEdge.InverterPort")))
	handler.Timeout = 10 * time.Second
	handler.SlaveId = 0x01
	err := handler.Connect()
	if err != nil {
		log.Error().Msgf("Error connecting to Inverter: %s", err.Error())
	}
	client := modbus.NewClient(handler)
	defer handler.Close()

	// Collect and log common inverter data
	infoData, err := client.ReadHoldingRegisters(40000, 70)
	cm, err := solaredge.NewCommonModel(infoData)
	log.Info().Msgf("Inverter Model: %s", cm.C_Model)
	log.Info().Msgf("Inverter Serial: %s", cm.C_SerialNumber)
	log.Info().Msgf("Inverter Version: %s", cm.C_Version)

	infoData2, err := client.ReadHoldingRegisters(40121, 65)
	cm2, err := solaredge.NewCommonMeter(infoData2)
	log.Info().Msgf("Meter Manufacturer: %s", cm2.C_Manufacturer)
	log.Info().Msgf("Meter Model: %s", cm2.C_Model)
	log.Info().Msgf("Meter Serial: %s", cm2.C_SerialNumber)
	log.Info().Msgf("Meter Version: %s", cm2.C_Version)
	log.Info().Msgf("Meter Option: %s", cm2.C_Option)
	// Collect logs forever
	for {
		inverterData, err := client.ReadHoldingRegisters(40069, 40)
		if err != nil {
			log.Error().Msgf("Error reading holding registers: %s", err.Error())
			log.Error().Msgf("Attempting to reconnect")
			_ = handler.Close()
			time.Sleep(7 * time.Second)
			_ = handler.Connect()
			continue
		}
		id, err := solaredge.NewInverterModel(inverterData)
		if err != nil {
			log.Error().Msgf("Error parsing data: %s", err.Error())
			continue
		}

		infoData3, err := client.ReadHoldingRegisters(40188, 105)
		mt, err := solaredge.NewMeterModel(infoData3)
		log.Debug().Msgf("Meter AC Current: %f", float64(mt.M_AC_Current)*math.Pow(10, float64(mt.M_AC_Current_SF)))
		log.Debug().Msgf("Meter VoltageLN: %f", float64(mt.M_AC_VoltageLN)*math.Pow(10, float64(mt.M_AC_Voltage_SF)))
		log.Debug().Msgf("Meter PF: %d", mt.M_AC_PF)
		log.Debug().Msgf("Meter Freq: %f", float64(mt.M_AC_Frequency)*math.Pow(10, float64(mt.M_AC_Frequency_SF)))
		log.Debug().Msgf("Meter AC Power: %f", float64(mt.M_AC_Power)*math.Pow(10.0, float64(mt.M_AC_Power_SF)))
		log.Debug().Msgf("Meter M_AC_VA: %f", float64(mt.M_AC_VA)*math.Pow(10.0, float64(mt.M_AC_VA_SF)))
		log.Debug().Msgf("Meter M_Exported: %f", float64(mt.M_Exported)*math.Pow(10.0, float64(mt.M_Energy_W_SF)))
		log.Debug().Msgf("Meter M_Imported: %f", float64(mt.M_Imported)*math.Pow(10.0, float64(mt.M_Energy_W_SF)))

		log.Debug().Msg("-------------------------------------------")
		log.Debug().Msg("Data retrieved from inverter")
		setMetrics(id)
		setMetricsForMeter(mt)
		time.Sleep(time.Duration(interval) * time.Second)
	}

}

func setMetrics(i solaredge.InverterModel) {
	exporter.SunSpec_DID.Set(float64(i.SunSpec_DID))
	exporter.SunSpec_Length.Set(float64(i.SunSpec_Length))
	exporter.AC_Current.Set(float64(i.AC_Current))
	exporter.AC_CurrentA.Set(float64(i.AC_CurrentA))
	exporter.AC_CurrentB.Set(float64(i.AC_CurrentB))
	exporter.AC_CurrentC.Set(float64(i.AC_CurrentC))
	exporter.AC_Current_SF.Set(float64(i.AC_Current_SF))
	exporter.AC_VoltageAB.Set(float64(i.AC_VoltageAB))
	exporter.AC_VoltageBC.Set(float64(i.AC_VoltageBC))
	exporter.AC_VoltageCA.Set(float64(i.AC_VoltageCA))
	exporter.AC_VoltageAN.Set(float64(i.AC_VoltageAN))
	exporter.AC_VoltageBN.Set(float64(i.AC_VoltageBN))
	exporter.AC_VoltageCN.Set(float64(i.AC_VoltageCN))
	exporter.AC_Voltage_SF.Set(float64(i.AC_Voltage_SF))
	exporter.AC_Power.Set(float64(i.AC_Power))
	exporter.AC_Power_SF.Set(float64(i.AC_Power_SF))
	exporter.AC_Frequency.Set(float64(i.AC_Frequency))
	exporter.AC_Frequency_SF.Set(float64(i.AC_Frequency_SF))
	exporter.AC_VA.Set(float64(i.AC_VA))
	exporter.AC_VA_SF.Set(float64(i.AC_VA_SF))
	exporter.AC_VAR.Set(float64(i.AC_VAR))
	exporter.AC_VAR_SF.Set(float64(i.AC_VAR_SF))
	exporter.AC_PF.Set(float64(i.AC_PF))
	exporter.AC_PF_SF.Set(float64(i.AC_PF_SF))
	exporter.AC_Energy_WH.Set(float64(i.AC_Energy_WH))
	exporter.AC_Energy_WH_SF.Set(float64(i.AC_Energy_WH_SF))
	exporter.DC_Current.Set(float64(i.DC_Current))
	exporter.DC_Current_SF.Set(float64(i.DC_Current_SF))
	exporter.DC_Voltage.Set(float64(i.DC_Voltage))
	exporter.DC_Voltage_SF.Set(float64(i.DC_Voltage_SF))
	exporter.DC_Power.Set(float64(i.DC_Power))
	exporter.DC_Power_SF.Set(float64(i.DC_Power_SF))
	exporter.Temp_Sink.Set(float64(i.Temp_Sink))
	exporter.Temp_SF.Set(float64(i.Temp_SF))
	exporter.Status.Set(float64(i.Status))
	exporter.Status_Vendor.Set(float64(i.Status_Vendor))
}

func setMetricsForMeter(m solaredge.MeterModel) {
	exporter.M_SunSpec_DID.Set(float64(m.SunSpec_DID))
	exporter.M_SunSpec_Length.Set(float64(m.SunSpec_Length))
	exporter.M_AC_Current.Set(float64(m.M_AC_Current))
	exporter.M_AC_CurrentA.Set(float64(m.M_AC_CurrentA))
	exporter.M_AC_CurrentB.Set(float64(m.M_AC_CurrentB))
	exporter.M_AC_CurrentC.Set(float64(m.M_AC_CurrentC))
	exporter.M_AC_Current_SF.Set(float64(m.M_AC_Current_SF))
	exporter.M_AC_VoltageLN.Set(float64(m.M_AC_VoltageLN))
	exporter.M_AC_VoltageAN.Set(float64(m.M_AC_VoltageAN))
	exporter.M_AC_VoltageBN.Set(float64(m.M_AC_VoltageBN))
	exporter.M_AC_VoltageCN.Set(float64(m.M_AC_VoltageCN))
	exporter.M_AC_VoltageLL.Set(float64(m.M_AC_VoltageLL))
	exporter.M_AC_VoltageAB.Set(float64(m.M_AC_VoltageAB))
	exporter.M_AC_VoltageBC.Set(float64(m.M_AC_VoltageBC))
	exporter.M_AC_VoltageCA.Set(float64(m.M_AC_VoltageCA))
	exporter.M_AC_Voltage_SF.Set(float64(m.M_AC_Voltage_SF))
	exporter.M_AC_Frequency.Set(float64(m.M_AC_Frequency))
	exporter.M_AC_Frequency_SF.Set(float64(m.M_AC_Frequency_SF))
	exporter.M_AC_Power.Set(float64(m.M_AC_Power))
	exporter.M_AC_Power_A.Set(float64(m.M_AC_Power_A))
	exporter.M_AC_Power_B.Set(float64(m.M_AC_Power_B))
	exporter.M_AC_Power_C.Set(float64(m.M_AC_Power_C))
	exporter.M_AC_Power_SF.Set(float64(m.M_AC_Power_SF))
	exporter.M_AC_VA.Set(float64(m.M_AC_VA))
	exporter.M_AC_VA_A.Set(float64(m.M_AC_VA_A))
	exporter.M_AC_VA_B.Set(float64(m.M_AC_VA_B))
	exporter.M_AC_VA_C.Set(float64(m.M_AC_VA_C))
	exporter.M_AC_VA_SF.Set(float64(m.M_AC_VA_SF))
	exporter.M_AC_VAR.Set(float64(m.M_AC_VAR))
	exporter.M_AC_VAR_A.Set(float64(m.M_AC_VAR_A))
	exporter.M_AC_VAR_B.Set(float64(m.M_AC_VAR_B))
	exporter.M_AC_VAR_C.Set(float64(m.M_AC_VAR_C))
	exporter.M_AC_VAR_SF.Set(float64(m.M_AC_VAR_SF))
	exporter.M_AC_PF.Set(float64(m.M_AC_PF))
	exporter.M_AC_PF_A.Set(float64(m.M_AC_PF_A))
	exporter.M_AC_PF_B.Set(float64(m.M_AC_PF_B))
	exporter.M_AC_PF_C.Set(float64(m.M_AC_PF_C))
	exporter.M_AC_PF_SF.Set(float64(m.M_AC_PF_SF))
	exporter.M_Exported.Set(float64(m.M_Exported))
	exporter.M_Exported_A.Set(float64(m.M_Exported_A))
	exporter.M_Exported_B.Set(float64(m.M_Exported_B))
	exporter.M_Exported_C.Set(float64(m.M_Exported_C))
	exporter.M_Imported.Set(float64(m.M_Imported))
	exporter.M_Imported_A.Set(float64(m.M_Imported_A))
	exporter.M_Imported_B.Set(float64(m.M_Imported_B))
	exporter.M_Imported_C.Set(float64(m.M_Imported_C))
	exporter.M_Energy_W_SF.Set(float64(m.M_Energy_W_SF))
}
