package design

import . "goa.design/goa/v3/dsl"

var _ = API("statuslist", func() {
	Title("XFSC Status List Service")
	Description("Status list REST API")
})

var _ = Service("status", func() {
	Description("Status list endpoints")

	Method("getList", func() {
		Payload(func() {
			Attribute("tenantId", String)
			Attribute("listId", Int)

			Required("tenantId", "listId")
		})

		Result(Any)

		HTTP(func() {
			GET("/status/{tenantId}/{listId}")
			Response(StatusOK)
			Response(StatusBadRequest)
			Response(StatusInternalServerError)
		})
	})

	Method("revoke", func() {
		Payload(func() {
			Attribute("tenantId", String)
			Attribute("listId", Int)
			Attribute("index", Int)

			Required("tenantId", "listId", "index")
		})

		Result(func() {
			Attribute("tenantId", String)
			Attribute("listId", Int)
			Attribute("index", Int)
			Attribute("status", String)

			Required("tenantId", "listId", "index", "status")
		})

		HTTP(func() {
			POST("/status/{tenantId}/{listId}/revoke/{index}")

			Response(StatusOK)
			Response(StatusInternalServerError)
		})
	})

	Method("health", func() {
		Result(String)

		HTTP(func() {
			GET("/health")
			Response(StatusOK)
		})
	})
})
