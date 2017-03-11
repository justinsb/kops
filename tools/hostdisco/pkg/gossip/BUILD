load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "client.go",
        "peer.go",
        "server.go",
    ],
    visibility = ["//visibility:public"],
    deps = [
        "//hostdisco/pkg/proto:go_default_library",
        "//vendor/google.golang.org/grpc:go_default_library",
    ],
)
