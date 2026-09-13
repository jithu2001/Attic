import 'dart:convert';

import 'package:attic/core/api/api_client.dart';
import 'package:attic/core/layout/window_size.dart';
import 'package:attic/core/providers.dart';
import 'package:attic/core/theme/app_theme.dart';
import 'package:attic/features/music/data/models.dart';
import 'package:attic/features/music/ui/artist_albums_screen.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

/// A dio adapter that answers with canned JSON, so the grid can be driven from
/// a realistic API response rather than hand-built model objects.
class CannedAdapter implements HttpClientAdapter {
  CannedAdapter(this.body, {this.status = 200});

  final Object body;
  final int status;
  final List<String> requestedPaths = <String>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requestedPaths.add(options.path);
    return ResponseBody.fromString(
      jsonEncode(body),
      status,
      headers: const <String, List<String>>{
        Headers.contentTypeHeader: <String>['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Map<String, dynamic> albumJson({
  required String id,
  required String name,
  int? year,
  int trackCount = 5,
}) =>
    <String, dynamic>{
      'id': id,
      'name': name,
      'album_artist': 'Miles Davis',
      'artist_id': 'TWlsZXMgRGF2aXM',
      'year': year,
      'track_count': trackCount,
      'duration_s': 2700.0,
      // Null so the test never reaches for the network to fetch artwork.
      'cover_url': null,
    };

/// Pumps [child] at [size] with the API backed by [adapter].
///
/// The view itself is resized rather than a MediaQuery being wrapped around the
/// tree: a GridView only builds what fits in the real viewport, so a pretend
/// size would leave off-screen cards unbuilt and make card counts measure the
/// window instead of the data.
Future<void> pumpScreen(
  WidgetTester tester, {
  required CannedAdapter adapter,
  required Size size,
  required Widget child,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);

  final client = ApiClient()..baseUrl = 'https://homeserver.tailnet.ts.net';
  client.dio.httpClientAdapter = adapter;

  await tester.pumpWidget(ProviderScope(
    overrides: <Override>[apiClientProvider.overrideWithValue(client)],
    child: MaterialApp(theme: AppTheme.light(null), home: child),
  ));
}

void main() {
  // Tall enough that a three-album grid is fully built: GridView only
  // constructs what is visible, and asserting on card count otherwise measures
  // the viewport rather than the data.
  const compact = Size(400, 1200);
  const expanded = Size(1280, 900);

  testWidgets('renders one card per album from the API response', (tester) async {
    final adapter = CannedAdapter(<String, dynamic>{
      'albums': <Map<String, dynamic>>[
        albumJson(id: 'a1', name: 'Kind of Blue', year: 1959),
        albumJson(id: 'a2', name: 'Bitches Brew', year: 1970),
        albumJson(id: 'a3', name: 'Milestones', year: 1958),
      ],
    });

    await pumpScreen(
      tester,
      adapter: adapter,
      size: compact,
      child: const ArtistAlbumsScreen(artistId: 'TWlsZXMgRGF2aXM'),
    );

    // First frame is the loading state.
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    await tester.pumpAndSettle();

    expect(adapter.requestedPaths.single,
        '/api/v1/music/artists/TWlsZXMgRGF2aXM/albums');
    expect(find.byType(AlbumCard), findsNWidgets(3));

    // Title and year, the two lines the design calls for.
    expect(find.text('Kind of Blue'), findsOneWidget);
    expect(find.text('1959'), findsOneWidget);
    expect(find.text('Bitches Brew'), findsOneWidget);
    expect(find.text('1970'), findsOneWidget);
  });

  testWidgets('uses the M3 type scale rather than ad-hoc sizes', (tester) async {
    final adapter = CannedAdapter(<String, dynamic>{
      'albums': <Map<String, dynamic>>[albumJson(id: 'a1', name: 'Kind of Blue', year: 1959)],
    });

    await pumpScreen(
      tester,
      adapter: adapter,
      size: compact,
      child: const ArtistAlbumsScreen(artistId: 'x'),
    );
    await tester.pumpAndSettle();

    final context = tester.element(find.byType(AlbumCard));
    final textTheme = Theme.of(context).textTheme;

    final title = tester.widget<Text>(find.text('Kind of Blue'));
    final year = tester.widget<Text>(find.text('1959'));

    // Hardcoded font sizes are the thing the design rules forbid; asserting
    // identity with the theme's styles is how that stays true.
    expect(title.style, textTheme.titleMedium);
    expect(year.style?.fontSize, textTheme.bodySmall?.fontSize);
  });

  testWidgets('shows an album with no year without crashing', (tester) async {
    final adapter = CannedAdapter(<String, dynamic>{
      'albums': <Map<String, dynamic>>[albumJson(id: 'a1', name: 'Untitled', year: null)],
    });

    await pumpScreen(
      tester,
      adapter: adapter,
      size: compact,
      child: const ArtistAlbumsScreen(artistId: 'x'),
    );
    await tester.pumpAndSettle();

    expect(find.text('Untitled'), findsOneWidget);
    expect(find.text('—'), findsOneWidget);
  });

  testWidgets('lays out two columns compact and four expanded', (tester) async {
    // The grid column count is the whole adaptive story for this screen, so it
    // is asserted directly rather than inferred from pixel positions.
    expect(AlbumGrid.columnsFor(WindowSizeClass.compact), 2);
    expect(AlbumGrid.columnsFor(WindowSizeClass.medium), 4);
    expect(AlbumGrid.columnsFor(WindowSizeClass.expanded), 4);

    final adapter = CannedAdapter(<String, dynamic>{
      'albums': <Map<String, dynamic>>[
        for (var i = 0; i < 4; i++) albumJson(id: 'a$i', name: 'Album $i', year: 1960 + i),
      ],
    });

    await pumpScreen(
      tester,
      adapter: adapter,
      size: expanded,
      child: const ArtistAlbumsScreen(artistId: 'x'),
    );
    await tester.pumpAndSettle();

    final grid = tester.widget<GridView>(find.byType(GridView));
    final delegate = grid.gridDelegate as SliverGridDelegateWithFixedCrossAxisCount;
    expect(delegate.crossAxisCount, 4);
  });

  testWidgets('shows an empty state when the artist has no albums', (tester) async {
    final adapter = CannedAdapter(<String, dynamic>{'albums': <Map<String, dynamic>>[]});

    await pumpScreen(
      tester,
      adapter: adapter,
      size: compact,
      child: const ArtistAlbumsScreen(artistId: 'x'),
    );
    await tester.pumpAndSettle();

    expect(find.byType(AlbumCard), findsNothing);
    expect(find.text('No albums'), findsOneWidget);
  });

  testWidgets('offers a retry when the API fails', (tester) async {
    final adapter = CannedAdapter(
      <String, dynamic>{
        'error': <String, dynamic>{'code': 'internal', 'message': 'Something went wrong.'}
      },
      status: 500,
    );

    await pumpScreen(
      tester,
      adapter: adapter,
      size: compact,
      child: const ArtistAlbumsScreen(artistId: 'x'),
    );
    await tester.pumpAndSettle();

    // The server's own message is what the user sees, not a generic one.
    expect(find.text('Something went wrong.'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Try again'), findsOneWidget);
  });

  testWidgets('survives a 1.3 system font scale without overflowing',
      (tester) async {
    // The design rules require the UI to hold up at 1.3x text, and a grid of
    // fixed-ratio cards is exactly where that tends to break.
    final adapter = CannedAdapter(<String, dynamic>{
      'albums': <Map<String, dynamic>>[
        albumJson(id: 'a1', name: 'A Very Long Album Title That Wraps', year: 1959),
        albumJson(id: 'a2', name: 'Bitches Brew', year: 1970),
      ],
    });

    tester.view.physicalSize = compact;
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final client = ApiClient()..baseUrl = 'https://homeserver.tailnet.ts.net';
    client.dio.httpClientAdapter = adapter;

    await tester.pumpWidget(ProviderScope(
      overrides: <Override>[apiClientProvider.overrideWithValue(client)],
      child: MediaQuery(
        data: const MediaQueryData(textScaler: TextScaler.linear(1.3)),
        child: MaterialApp(
          theme: AppTheme.light(null),
          home: const ArtistAlbumsScreen(artistId: 'x'),
        ),
      ),
    ));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.byType(AlbumCard), findsNWidgets(2));
  });

  test('albums parse from the API shape', () {
    final album = Album.fromJson(albumJson(id: 'a1', name: 'Kind of Blue', year: 1959));

    expect(album.id, 'a1');
    expect(album.albumArtist, 'Miles Davis');
    expect(album.year, 1959);
    expect(album.durationS, 2700.0);
    expect(album.coverUrl, isNull);
  });
}
