import { useState } from 'react';
import type { SavedMovie, WatchStatus } from '../types';
import { SavedCard } from './MovieCard';

interface Props {
    hidden: boolean;
    library: SavedMovie[];
    onUpdate: (movie: SavedMovie, changes: Partial<SavedMovie>) => void;
    onRemove: (movie: SavedMovie) => void;
}

export function LibraryPanel({ hidden, library, onUpdate, onRemove }: Props) {
    const [filter, setFilter] = useState<WatchStatus | 'all'>('all');
    const items = library.filter(movie => filter === 'all' || movie.status === filter);

    return (
        <section id="library-panel" hidden={hidden}>
            <div className="section-heading">
                <h2>私の映画棚</h2>
                <select aria-label="登録作品の絞り込み" value={filter} onChange={event => setFilter(event.target.value as WatchStatus | 'all')}>
                    <option value="all">すべて</option>
                    <option value="want">観たい</option>
                    <option value="watched">鑑賞済み</option>
                </select>
            </div>
            <p className="muted">記録はこのブラウザに保存されます。</p>
            <div className="grid">
                {items.map(movie => <SavedCard key={`${movie.demo ? 'demo' : 'tmdb'}-${movie.id}`} movie={movie} onUpdate={onUpdate} onRemove={onRemove} />)}
                {!items.length && <p className="empty">まだ映画がありません。気になる一本を検索して登録しましょう。</p>}
            </div>
        </section>
    );
}
