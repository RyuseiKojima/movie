import type { ReactNode } from 'react';
import type { Movie, SavedMovie, WatchStatus } from '../types';

function Poster({ movie }: { movie: Movie }) {
    if (movie.poster_path && /^\/[a-zA-Z0-9._-]+$/.test(movie.poster_path)) {
        return (
            <div className="poster">
                <img src={`https://image.tmdb.org/t/p/w342${movie.poster_path}`} alt={`${movie.title}のポスター`} loading="lazy" />
            </div>
        );
    }
    return <div className="poster"><span>◉</span><p>{movie.title}</p></div>;
}

function MovieCard({ movie, children }: { movie: Movie; children: ReactNode }) {
    return (
        <article className="card">
            <Poster movie={movie} />
            <div className="card-content">
                <p className="muted">{movie.release_date?.slice(0, 4) || '公開年不明'}{movie.demo ? ' · デモ作品' : ''}</p>
                <h3>{movie.title}</h3>
                <p className="overview">{movie.overview || 'あらすじはまだありません。'}</p>
                {children}
            </div>
        </article>
    );
}

interface CandidateCardProps {
    movie: Movie;
    saved: boolean;
    onAdd: (movie: Movie) => void;
}

export function CandidateCard({ movie, saved, onAdd }: CandidateCardProps) {
    return (
        <MovieCard movie={movie}>
            <button className="add" disabled={saved} onClick={() => onAdd(movie)}>
                {saved ? '✓ 登録済み' : '＋ 観たい映画に登録'}
            </button>
        </MovieCard>
    );
}

interface SavedCardProps {
    movie: SavedMovie;
    onUpdate: (movie: SavedMovie, changes: Partial<SavedMovie>) => void;
    onRemove: (movie: SavedMovie) => void;
}

export function SavedCard({ movie, onUpdate, onRemove }: SavedCardProps) {
    return (
        <MovieCard movie={movie}>
            <select aria-label={`${movie.title}の鑑賞状態`} value={movie.status} onChange={event => onUpdate(movie, { status: event.target.value as WatchStatus })}>
                <option value="want">観たい</option>
                <option value="watched">鑑賞済み</option>
            </select>
            <select aria-label={`${movie.title}の評価`} value={movie.rating || 0} onChange={event => onUpdate(movie, { rating: Number(event.target.value) })}>
                {[0, 1, 2, 3, 4, 5].map(i => <option key={i} value={i}>{i ? '★'.repeat(i) : '評価なし'}</option>)}
            </select>
            <textarea
                aria-label={`${movie.title}のメモ`}
                placeholder="感想や覚えておきたいこと"
                maxLength={2000}
                value={movie.note || ''}
                onChange={event => onUpdate(movie, { note: event.target.value })}
            />
            <button className="remove" onClick={() => {
                if (confirm(`「${movie.title}」を映画棚から削除しますか？`)) onRemove(movie);
            }}>棚から削除</button>
        </MovieCard>
    );
}
