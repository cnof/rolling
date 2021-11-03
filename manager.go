package rolling

// Option defined config option
type Option func(*Logger)

func WithLogPath(path string) Option {
	return func(logger *Logger) {
		logger.LogPath = path
	}
}

func WithFilename(name string) Option {
	return func(logger *Logger) {
		logger.Filename = name
	}
}

func WithMaxAge(maxAge int) Option {
	return func(logger *Logger) {
		logger.MaxAge = maxAge
	}
}

func WithMaxRemain(maxRemain int) Option {
	return func(logger *Logger) {
		logger.MaxRemain = maxRemain
	}
}

func WithMaxSize(maxSize int) Option {
	return func(logger *Logger) {
		logger.MaxSize = maxSize
	}
}

func WithTimeRolling() Option {
	return func(logger *Logger) {
		logger.RollingPolicy = TimeRolling
	}
}

func WithTimePattern(timePattern string) Option {
	return func(logger *Logger) {
		logger.TimePattern = timePattern
	}
}

func WithCompress() Option {
	return func(logger *Logger) {
		logger.Compress = true
	}
}

func WithLocalTime() Option {
	return func(logger *Logger) {
		logger.LocalTime = true
	}
}
