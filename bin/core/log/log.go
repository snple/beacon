package log

import (
	"time"

	"github.com/DeRuina/timberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func Init(debug bool, logFile string) {
	if debug {
		Logger, _ = zap.NewDevelopment()
	} else {
		tjLogger := &timberjack.Logger{
			Filename:           logFile,                    // Choose an appropriate path
			MaxSize:            50,                         // megabytes
			MaxBackups:         3,                          // backups
			MaxAge:             28,                         // days
			Compression:        "zstd",                     // "none" | "gzip" | "zstd" (preferred over legacy Compress)
			LocalTime:          true,                       // default: false (use UTC)
			RotationInterval:   24 * time.Hour,             // Rotate daily if no other rotation met
			RotateAtMinutes:    []int{0, 15, 30, 45},       // Also rotate at HH:00, HH:15, HH:30, HH:45
			RotateAt:           []string{"00:00", "12:00"}, // Also rotate at 00:00 and 12:00 each day
			BackupTimeFormat:   "2006-01-02-15-04-05",      // Rotated files will have format <logfilename>-2006-01-02-15-04-05-<reason>.log
			AppendTimeAfterExt: true,                       // put timestamp after ".log" (foo.log-<timestamp>-<reason>)
			FileMode:           0o644,                      // Custom permissions for newly created files. If unset or 0, defaults to 640.
		}

		// 集成到 zap 核心
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(tjLogger),
			zap.InfoLevel,
		)
		Logger = zap.New(core)
	}
}

func Sync() {
	Logger.Sync()
}
