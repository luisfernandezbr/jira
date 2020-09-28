.PHONY: gen

gen:
	@easyjson -all -lower_camel_case -pkg ./internal
