# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic
Versioning](http://semver.org/spec/v2.0.0.html).

## 2.0.0 - 2020-04-25

### Added

- Major rewrite based on the excellent [sensu-plugin-sdk][sdk]! 

- Added new `--help` output (we get this for free w/ `sensu-plugin-sdk`)

- Added new GitHub Actions CI (we get this for free w/ `sensu-plugin-sdk`)

- Added support for Sensu API Keys! (no more username & password auth) 

- Updated readme w/ new usage examples and configuration attributes

### Changed 

- **BREAKING CHANGE:** removed support for username & password auth. 
  Please update your templates to use `--sensu-api-key` or `$SENSU_API_KEY`.

- Refactored the old mega function into distinct "parse", "match", and 
  "process" functions.  

## 1.0.0 – 2019-11-08

### Added

- Updated readme with configuration details (environment variables)

- Added "remediation action" documentation 

### Changed 

- **BREAKING CHANGE:** Renamed project & binary from: 
  `sensu-go-remediation-handler`, to: 
  `sensu-remediation-handler`

- **BREAKING CHANGE:** Changed remediation action annotation, from:
  `sensu.io/plugins/remediation/config/actions`, to:
  `io.sensu.remediation.config.actions`

  Please update your templates accordingly! 

### Fixed 



[sdk]: https://github.com/sensu-community/sensu-plugin-sdk 