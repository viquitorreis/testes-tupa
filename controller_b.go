package main

func ContrBRoutes() []RouteInfo {
	return []RouteInfo{
		{
			Path:    "/contrB",
			Method:  "POST",
			Handler: handleSendString,
		},
	}
}
