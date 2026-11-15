# SuperStar mobile adaptation

Source: https://github.com/842720301wh-ship-it/SuperStar
Commit: f51cf91768a72d6e88382591577f73f93579e427

Core api/ and superstar.py preserve upstream authorship headers. Mobile changes: memory-only cookies/logging, stdlib HTML parsing, protocol client adapter, bounded cancellation/progress integration, and a cancellable chapter scheduler. No upstream credentials or workflow configuration are bundled.

The pure-Python pyaes 1.6.1 dependency is vendored with its MIT license in pyaes/LICENSE.txt. Chaquopy supplies Python 3.12 on Android. The mobile model adapter supports Chat Completions, Responses and Anthropic Messages without requiring the upstream OpenAI SDK.
