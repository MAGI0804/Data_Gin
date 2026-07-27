package global

// MallWeatherEnabledAtStartup is captured after configuration validation.
// Weather workers, schedulers, dispatchers, and HTTP admission must all use
// the same value because their lifecycle changes require a process restart.
var MallWeatherEnabledAtStartup bool
