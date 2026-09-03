package design

import . "goa.design/goa/v3/dsl"

var _ = API("statuslist", func() {
	Title("XFSC Status List Service")
	Description("Status list REST API")

	Server("statuslist", func() {
		Host("default", func() {
			URI("http://localhost:8080")
		})
	})
})

var StatusListResult = ResultType("application/vnd.xfsc.status-list", func() {
	TypeName("StatusListResult")

	Attributes(func() {
		Attribute("tenantId", String)
		Attribute("listId", Int)
		Attribute("list", String)
		Required("tenantId", "listId", "list")
	})

	View("default", func() {
		Attribute("tenantId")
		Attribute("listId")
		Attribute("list")
	})
})

var RevokeResult = ResultType("application/vnd.xfsc.status-list-revoke", func() {
	TypeName("RevokeResult")

	Attributes(func() {
		Attribute("tenantId", String)
		Attribute("listId", Int)
		Attribute("index", Int)
		Attribute("status", String)
		Required("tenantId", "listId", "index", "status")
	})

	View("default", func() {
		Attribute("tenantId")
		Attribute("listId")
		Attribute("index")
		Attribute("status")
	})
})

var ServiceError = ResultType("application/vnd.xfsc.error", func() {
	TypeName("ErrorResult")

	Attributes(func() {
		Attribute("message", String)
		Attribute("status", Int)
		Required("message", "status")
	})

	View("default", func() {
		Attribute("message")
		Attribute("status")
	})
})

var _ = Service("status", func() {
	Description("Status list endpoints")

	Error("bad_request", ServiceError)
	Error("internal_error", ServiceError)

	Method("health", func() {
		Result(String)

		HTTP(func() {
			GET("/health")

			Response(StatusOK)
		})
	})

	Method("getList", func() {
		Description("Returns a status list as JSON, JWT status list, or StatusList2021 credential depending on Content-Type.")

		Payload(func() {
			Attribute("tenantId", String)
			Attribute("listId", Int)
			Attribute("accept", String)
			Attribute("groupId", String)

			Required("tenantId", "listId")
		})

		Result(Any)

		HTTP(func() {
			GET("/v1/tenants/{tenantId}/status/{listId}")

			Param("tenantId")
			Param("listId")

			Header("accept:Accept")
			Header("groupId:X-Group-Id")

			Response(StatusOK, func() {
				ContentType("application/statuslist+jwt")
			})
		})
	})

	Method("revoke", func() {
		Description("Revokes a credential by setting the bit at the given index.")

		Payload(func() {
			Attribute("tenantId", String)
			Attribute("listId", Int)
			Attribute("index", Int)

			Required("tenantId", "listId", "index")
		})

		Result(RevokeResult)

		HTTP(func() {
			POST("/status/{tenantId}/{listId}/revoke/{index}")

			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
	})
})
