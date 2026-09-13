import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/providers.dart';

/// Album art, cached on disk and keyed by content hash.
///
/// The URL carries a media token, which rotates; the cache key deliberately
/// does not, so rotating a token does not throw away the cached artwork for a
/// whole library.
class CoverImage extends ConsumerWidget {
  const CoverImage({
    super.key,
    required this.coverUrl,
    this.size,
    this.borderRadius = 12,
  });

  /// The server-relative cover URL, or null when a track has no artwork.
  final String? coverUrl;
  final double? size;
  final double borderRadius;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final radius = BorderRadius.circular(borderRadius);
    final path = coverUrl;

    if (path == null) {
      return ClipRRect(borderRadius: radius, child: _Fallback(size: size));
    }

    final url = ref.watch(authRepositoryProvider).signMediaUrl(path);

    return ClipRRect(
      borderRadius: radius,
      child: CachedNetworkImage(
        imageUrl: url,
        cacheKey: path,
        width: size,
        height: size,
        fit: BoxFit.cover,
        fadeInDuration: const Duration(milliseconds: 150),
        placeholder: (context, _) => _Fallback(size: size),
        errorWidget: (context, _, __) => _Fallback(size: size),
      ),
    );
  }
}

class _Fallback extends StatelessWidget {
  const _Fallback({this.size});

  final double? size;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Container(
      width: size,
      height: size,
      color: colors.surfaceContainerHighest,
      child: Center(
        child: Icon(
          Icons.album_outlined,
          size: size == null ? 32 : size! * 0.4,
          color: colors.onSurfaceVariant,
        ),
      ),
    );
  }
}
