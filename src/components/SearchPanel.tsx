import { useEffect, useRef, useState, type FormEvent } from 'react';
import { fetchConfig, searchMovies } from '../api';
import type { Movie } from '../types';
import { CandidateCard } from './MovieCard';

interface Props {
    hidden: boolean;
    notify: (message: string) => void;
    onConfig: (demo: boolean) => void;
    isSaved: (movie: Movie) => boolean;
    onAdd: (movie: Movie) => void;
}

export function SearchPanel({ hidden, notify, onConfig, isSaved, onAdd }: Props) {
    const [query, setQuery] = useState('');
    const [title, setTitle] = useState('いま注目の映画');
    const [results, setResults] = useState<Movie[]>([]);
    const [loading, setLoading] = useState(false);
    const started = useRef(false);

    async function search(text: string) {
        setLoading(true);
        notify('映画を探しています…');
        try {
            const data = await searchMovies(text);
            setTitle(text.trim() ? '検索結果' : 'いま注目の映画');
            setResults(data.results.map(movie => ({ ...movie, demo: Boolean(data.demo) })));
            notify(data.demo ? 'デモモード：架空のサンプル作品を表示しています。実際の映画検索にはTMDBのAPIキーを設定してください。' : `${data.results.length}件の映画が見つかりました。`);
        } catch (error) { notify((error as Error).message); }
        finally { setLoading(false); }
    }

    useEffect(() => {
        if (started.current) return;
        started.current = true;
        fetchConfig().then(config => { onConfig(config.demo); search(''); }).catch((error: Error) => notify(error.message));
    }, []); // eslint-disable-line react-hooks/exhaustive-deps

    function handleSubmit(event: FormEvent) {
        event.preventDefault();
        search(query);
    }

    return (
        <section id="search-panel" hidden={hidden}>
            <form onSubmit={handleSubmit}>
                <label className="sr-only" htmlFor="query">映画タイトル</label>
                <input id="query" placeholder="映画タイトルで検索…" maxLength={200} value={query} onChange={event => setQuery(event.target.value)} />
                <button className="primary" disabled={loading}>検索する</button>
            </form>
            <h2>{title}</h2>
            <div className="grid">
                {results.map(movie => <CandidateCard key={movie.id} movie={movie} saved={isSaved(movie)} onAdd={onAdd} />)}
            </div>
        </section>
    );
}
