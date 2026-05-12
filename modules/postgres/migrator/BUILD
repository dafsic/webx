load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "migrator",
    srcs = ["module.go"],
    importpath = "compass.com/postgres/migrator",
    visibility = ["//visibility:public"],
    deps = [
        "//src/go/compass.com/app",
        "//src/go/compass.com/app/resources",
        "//src/go/compass.com/postgres",
        "//src/go/compass.com/postgres/pgdumpschema",
        "@com_github_pkg_errors//:errors",
        "@com_github_rubenv_sql_migrate//:sql-migrate",
    ],
)

go_test(
    name = "migrator_test",
    srcs = ["module_test.go"],
    embed = [":migrator"],
    deps = [
        "@com_github_mattn_go_sqlite3//:go-sqlite3",
        "@com_github_rubenv_sql_migrate//:sql-migrate",
        "@com_github_stretchr_testify//require",
        "@com_github_stretchr_testify//suite",
        "@in_gopkg_gorp_v1//:gorp_v1",
    ],
)
