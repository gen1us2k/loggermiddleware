# loggermiddleware
Custom logger middleware for echo framework

Implements nginx like logs for echo framework

### Usage

```Go
	e := echo.New()
	e.Use(loggermiddleware.CustomLogger())
```
