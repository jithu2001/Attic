import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../data/models.dart';
import '../music_providers.dart';
import 'cover_image.dart';

/// The library search bar.
///
/// `SearchAnchor.bar` is the M3 pattern: a bar that expands into a full-screen
/// view of suggestions. Results come back mixed — artists, albums and tracks —
/// so they are grouped under titleSmall headers rather than presented as one
/// undifferentiated list.
class MusicSearchField extends ConsumerWidget {
  const MusicSearchField({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return SearchAnchor.bar(
      barHintText: 'Search artists, albums and tracks',
      barLeading: const Icon(Icons.search),
      suggestionsBuilder: (context, controller) async {
        final query = controller.text.trim();
        if (query.isEmpty) return const <Widget>[];

        final results = await ref.read(searchProvider(query).future);
        if (!context.mounted) return const <Widget>[];
        if (results.isEmpty) {
          return <Widget>[
            const ListTile(
              leading: Icon(Icons.search_off),
              title: Text('No matches'),
            ),
          ];
        }
        return _grouped(context, results, controller);
      },
    );
  }

  List<Widget> _grouped(
    BuildContext context,
    List<SearchResult> results,
    SearchController controller,
  ) {
    final widgets = <Widget>[];

    for (final kind in SearchResultKind.values) {
      final section = results.where((r) => r.kind == kind).toList(growable: false);
      if (section.isEmpty) continue;

      widgets.add(_SectionHeader(label: _labelFor(kind, section.length)));
      widgets.addAll(section.map((result) => _ResultTile(
            result: result,
            onTap: () {
              controller.closeView(null);
              _open(context, result);
            },
          )));
    }
    return widgets;
  }

  static String _labelFor(SearchResultKind kind, int count) => switch (kind) {
        SearchResultKind.artist => count == 1 ? 'Artist' : 'Artists',
        SearchResultKind.album => count == 1 ? 'Album' : 'Albums',
        SearchResultKind.track => count == 1 ? 'Track' : 'Tracks',
      };

  static void _open(BuildContext context, SearchResult result) {
    switch (result.kind) {
      case SearchResultKind.artist:
        context.go('/music/artists/${result.id}');
      case SearchResultKind.album:
        context.go('/music/albums/${result.id}');
      case SearchResultKind.track:
        // A track's home is its album, which is where it can be played in
        // context rather than on its own.
        if (result.albumId.isNotEmpty) context.go('/music/albums/${result.albumId}');
    }
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
      child: Text(
        label,
        style: Theme.of(context).textTheme.titleSmall?.copyWith(
              color: Theme.of(context).colorScheme.primary,
            ),
      ),
    );
  }
}

class _ResultTile extends StatelessWidget {
  const _ResultTile({required this.result, required this.onTap});

  final SearchResult result;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: switch (result.kind) {
        SearchResultKind.artist => CircleAvatar(
            child: Text(result.name.isEmpty ? '?' : result.name.substring(0, 1).toUpperCase()),
          ),
        _ => SizedBox(
            width: 48,
            height: 48,
            child: CoverImage(coverUrl: result.coverUrl, size: 48, borderRadius: 8),
          ),
      },
      title: Text(result.name, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: result.subtitle.isEmpty
          ? null
          : Text(result.subtitle, maxLines: 1, overflow: TextOverflow.ellipsis),
      onTap: onTap,
    );
  }
}
