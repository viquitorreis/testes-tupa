package main

import "fmt"

var allRoutes []RouteInfo

func ContrARoutes() []RouteInfo {
	return []RouteInfo{
		{
			Path:        "/contrA",
			Method:      "GET",
			Handler:     handleSendString,
			Middlewares: nil,
		},
		{
			Path:        "/contrA",
			Method:      "POST",
			Handler:     handleSendString,
			Middlewares: nil,
		},
	}
}

func AddRoutes(groupMiddlewares MiddlewareChain, routeFuncs ...func() []RouteInfo) {
	for _, routeFunc := range routeFuncs {
		routes := routeFunc()
		for i := range routes {
			routes[i].Middlewares = append(routes[i].Middlewares, groupMiddlewares...)
		}
		allRoutes = append(allRoutes, routes...)
	}
}

func PrintRoutess() {
	for _, route := range allRoutes {
		fmt.Printf("ROUTEsss => Path: %s, Method: %s, Handler: %v, Middlewares: %v\n", route.Path, route.Method, route.Handler, route.Middlewares)
	}
}

func GetRoutes() []RouteInfo {
	routesCopy := make([]RouteInfo, len(allRoutes))
	copy(routesCopy, allRoutes)
	return routesCopy
}

func ExampleRouteManager() {
	AddRoutes(MiddlewareChain{MiddlewareContrA}, ContrARoutes)

	AddRoutes(MiddlewareChain{MiddlewareContrB}, ContrBRoutes)

	AddRoutes(MiddlewareChain{MiddlewareContrC}, ContrCRoutes)
}

func ContrCRoutes() []RouteInfo {
	return []RouteInfo{
		{
			Path:    "/contrC",
			Method:  "POST",
			Handler: handleSendString,
		},
	}
}
