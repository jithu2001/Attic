import 'dart:convert';

import 'package:attic/core/api/api_client.dart';
import 'package:attic/core/api/api_error.dart';
import 'package:attic/core/auth/auth_repository.dart';
import 'package:attic/core/auth/token_store.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// An in-memory SessionStore, standing in for the platform keystore.
class FakeSessionStore implements SessionStore {
  FakeSessionStore({StoredSession? initial}) : _session = initial;

  StoredSession? _session;
  String? _serverUrl;
  int clearCount = 0;

  StoredSession? get session => _session;

  @override
  Future<StoredSession?> read() async => _session;

  @override
  Future<String?> readServerUrl() async => _serverUrl ?? _session?.serverUrl;

  @override
  Future<void> writeServerUrl(String url) async => _serverUrl = url;

  @override
  Future<void> write(StoredSession session) async {
    _session = session;
    _serverUrl = session.serverUrl;
  }

  @override
  Future<void> writeTokens({
    required String accessToken,
    required String refreshToken,
  }) async {
    _session = _session?.copyWith(accessToken: accessToken, refreshToken: refreshToken);
  }

  @override
  Future<void> clearSession() async {
    clearCount++;
    _session = null;
  }
}

/// One recorded request, so a test can assert what the interceptor sent.
class RecordedRequest {
  RecordedRequest(this.path, this.authorization, this.body);

  final String path;
  final String? authorization;
  final Object? body;
}

/// A canned server. Each handler is looked up by path and may answer
/// differently on each call, which is how "401 then 200" is expressed.
class FakeAdapter implements HttpClientAdapter {
  FakeAdapter(this.handlers);

  final Map<String, ResponseBody Function(RequestOptions options, int call)> handlers;
  final List<RecordedRequest> requests = <RecordedRequest>[];
  final Map<String, int> _calls = <String, int>{};

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final path = Uri.parse(options.path).path;
    requests.add(RecordedRequest(
      path,
      options.headers['Authorization'] as String?,
      options.data,
    ));

    final handler = handlers[path];
    if (handler == null) {
      return ResponseBody.fromString('{}', 404, headers: _jsonHeaders);
    }
    final call = _calls.update(path, (n) => n + 1, ifAbsent: () => 0);
    return handler(options, call);
  }

  @override
  void close({bool force = false}) {}
}

const _jsonHeaders = <String, List<String>>{
  Headers.contentTypeHeader: <String>['application/json'],
};

ResponseBody _json(Object body, int status) =>
    ResponseBody.fromString(jsonEncode(body), status, headers: _jsonHeaders);

ResponseBody _unauthorized() => _json({
      'error': {'code': 'token_expired', 'message': 'Access token expired.'}
    }, 401);

/// Builds a repository whose client talks to [adapter].
({AuthRepository auth, FakeSessionStore store}) buildRepository(
  FakeAdapter adapter, {
  StoredSession? session,
}) {
  final client = ApiClient()..baseUrl = 'https://homeserver.tailnet.ts.net';
  client.dio.httpClientAdapter = adapter;

  final store = FakeSessionStore(initial: session);
  final auth = AuthRepository(client: client, store: store);

  // The retry client is built lazily from the main client's options, so it
  // needs the same adapter.
  auth.client.dio.options.baseUrl = 'https://homeserver.tailnet.ts.net';
  return (auth: auth, store: store);
}

const _session = StoredSession(
  serverUrl: 'https://homeserver.tailnet.ts.net',
  accessToken: 'old-access',
  refreshToken: 'old-refresh',
);

void main() {
  group('AuthInterceptor', () {
    test('attaches the access token to API requests', () async {
      final adapter = FakeAdapter({
        '/api/v1/music/artists': (_, __) => _json({'artists': []}, 200),
      });
      final built = buildRepository(adapter, session: _session);
      await built.auth.restore();

      await built.auth.client.getJson('/music/artists');

      expect(adapter.requests.single.authorization, 'Bearer old-access');
    });

    test('refreshes once on a 401 and replays the original request', () async {
      final adapter = FakeAdapter({
        // Fails the first time, succeeds once the token has been rotated.
        '/api/v1/music/artists': (options, call) => call == 0
            ? _unauthorized()
            : _json({
                'artists': [
                  {'id': 'x', 'name': 'Miles Davis', 'album_count': 1, 'track_count': 5}
                ]
              }, 200),
        '/api/v1/auth/refresh': (_, __) => _json({
              'access_token': 'new-access',
              'refresh_token': 'new-refresh',
              'token_type': 'Bearer',
              'user': {'id': 'u', 'username': 'ada', 'role': 'member'},
            }, 200),
      });
      final built = buildRepository(adapter, session: _session);
      await built.auth.restore();
      built.auth.dioRetryClient.httpClientAdapter = adapter;

      final json = await built.auth.client.getJson('/music/artists');

      expect((json['artists'] as List).length, 1);

      final paths = adapter.requests.map((r) => r.path).toList();
      expect(paths, <String>[
        '/api/v1/music/artists',
        '/api/v1/auth/refresh',
        '/api/v1/music/artists',
      ]);
      // The replay must carry the *new* token, not the one that just failed.
      expect(adapter.requests.last.authorization, 'Bearer new-access');

      // And the rotated pair must be persisted, or the next launch signs in
      // with a refresh token the server has already invalidated.
      expect(built.store.session?.accessToken, 'new-access');
      expect(built.store.session?.refreshToken, 'new-refresh');
    });

    test('signs out when the refresh token is rejected', () async {
      var sessionLost = false;

      final adapter = FakeAdapter({
        '/api/v1/music/artists': (_, __) => _unauthorized(),
        '/api/v1/auth/refresh': (_, __) => _json({
              'error': {'code': 'unauthorized', 'message': 'This session has expired.'}
            }, 401),
      });
      final built = buildRepository(adapter, session: _session);
      built.auth.onSessionLost = () => sessionLost = true;
      await built.auth.restore();
      built.auth.dioRetryClient.httpClientAdapter = adapter;

      await expectLater(
        built.auth.client.getJson('/music/artists'),
        throwsA(isA<ApiError>()),
      );

      expect(sessionLost, isTrue, reason: 'the app must be told to sign out');
      expect(built.store.clearCount, greaterThan(0),
          reason: 'stored tokens must be discarded');
    });

    test('does not retry more than once', () async {
      final adapter = FakeAdapter({
        // A server that answers 401 no matter how fresh the token is would
        // otherwise loop forever.
        '/api/v1/music/artists': (_, __) => _unauthorized(),
        '/api/v1/auth/refresh': (_, __) => _json({
              'access_token': 'new-access',
              'refresh_token': 'new-refresh',
            }, 200),
      });
      final built = buildRepository(adapter, session: _session);
      await built.auth.restore();
      built.auth.dioRetryClient.httpClientAdapter = adapter;

      await expectLater(
        built.auth.client.getJson('/music/artists'),
        throwsA(isA<ApiError>()),
      );

      final artistCalls =
          adapter.requests.where((r) => r.path == '/api/v1/music/artists').length;
      expect(artistCalls, 2, reason: 'one original attempt plus one replay');
    });

    test('never sends a token to the login endpoint', () async {
      final adapter = FakeAdapter({
        '/api/v1/auth/login': (_, __) => _json({
              'access_token': 'a',
              'refresh_token': 'r',
              'user': {'id': 'u', 'username': 'ada', 'role': 'member'},
            }, 200),
      });
      final built = buildRepository(adapter, session: _session);
      await built.auth.restore();

      await built.auth.signIn(username: 'ada', password: 'pw', deviceName: 'Pixel');

      expect(adapter.requests.single.authorization, isNull);
    });

    test('keeps the session when a refresh fails for network reasons', () async {
      final adapter = FakeAdapter({
        '/api/v1/music/artists': (_, __) => _unauthorized(),
        '/api/v1/auth/refresh': (options, __) => throw DioException.connectionError(
              requestOptions: options,
              reason: 'tailnet unreachable',
            ),
      });
      final built = buildRepository(adapter, session: _session);
      await built.auth.restore();
      built.auth.dioRetryClient.httpClientAdapter = adapter;

      await expectLater(
        built.auth.client.getJson('/music/artists'),
        throwsA(isA<ApiError>()),
      );

      // Being off the tailnet must not log someone out of their own server.
      expect(built.store.clearCount, 0);
      expect(built.store.session?.refreshToken, 'old-refresh');
    });
  });

  group('ApiClient', () {
    test('normalises a bare hostname into an https base URL', () {
      expect(
        ApiClient.normalizeServerUrl('homeserver.tailnet.ts.net/'),
        'https://homeserver.tailnet.ts.net',
      );
    });

    test('keeps an explicit scheme and strips trailing slashes', () {
      expect(
        ApiClient.normalizeServerUrl('http://192.168.1.10:8080//'),
        'http://192.168.1.10:8080',
      );
    });
  });
}
