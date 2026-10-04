import { useEffect, useRef, useState, type FormEvent } from 'react';
import { fetchConfig, recommendMovie, searchMovies } from '../api';
import type { Movie, SavedMovie, YearRange } from '../types';
import { CandidateCard } from './MovieCard';

interface Props {
    hidden: boolean;
    library: SavedMovie[];
    notify: (message: string) => void;
    onConfig: (demo: boolean) => void;
    isSaved: (movie: Movie) => boolean;
    onAdd: (movie: Movie) => void;
}

const toYear = (value: string) => value ? Number(value) : null;

export function SearchPanel({ hidden, library, notify, onConfig, isSaved, onAdd }: Props) {
    const [query, setQuery] = useState('');
    const [yearFrom, setYearFrom] = useState('');
    const [yearTo, setYearTo] = useState('');
    const [title, setTitle] = useState('いま注目の映画');
    const [results, setResults] = useState<Movie[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const [demo, setDemo] = useState(false);
    const [jev, setJev] = useState(false);
    const [mood, setMood] = useState('');
    const [suggestion, setSuggestion] = useState<Movie | null>(null);
    const [recommending, setRecommending] = useState(false);
    const [recommendError, setRecommendError] = useState('');
    const started = useRef(false);

    async function search(text: string, years: YearRange) {
        setLoading(true);
        setError('');
        setSuggestion(null);
        setRecommendError('');
        notify('');
        try {
            const data = await searchMovies(text, years);
            setTitle(text.trim() ? '検索結果' : 'いま注目の映画');
            setResults(data.results.map(movie => ({ ...movie, demo: Boolean(data.demo) })));
            setDemo(Boolean(data.demo));
            notify(data.demo ? 'デモモード：架空のサンプル作品を表示しています。実際の映画検索にはTMDBのAPIキーを設定してください。' : `${data.results.length}件の映画が見つかりました。`);
        } catch (error) {
            setResults([]);
            setError((error as Error).message);
        } finally { setLoading(false); }
    }

    useEffect(() => {
        if (started.current) return;
        started.current = true;
        fetchConfig().then(config => {
            onConfig(config.demo);
            setJev(config.jev);
            search('', { yearFrom: null, yearTo: null });
        }).catch((error: Error) => {
            setError(error.message);
            setLoading(false);
        });
    }, []); // eslint-disable-line react-hooks/exhaustive-deps

    function handleSearch(event: FormEvent) {
        event.preventDefault();
        search(query, { yearFrom: toYear(yearFrom), yearTo: toYear(yearTo) });
    }

    async function handleRecommend(event: FormEvent) {
        event.preventDefault();
        setRecommending(true);
        setRecommendError('');
        setSuggestion(null);
        notify('');
        try {
            const result = await recommendMovie(mood, library, results);
            setSuggestion({ ...result.movie, demo });
            notify(`Jevが選んだ一本です。モデルの確信度：${Math.round(result.confidence * 100)}%（満足度の保証ではありません）`);
        } catch (error) { setRecommendError((error as Error).message); }
        finally { setRecommending(false); }
    }

    const unsavedCount = results.filter(movie => !isSaved(movie)).length;

    return (
        <section id="search-panel" hidden={hidden}>
            <form className="search-form" onSubmit={handleSearch}>
                <label className="sr-only" htmlFor="query">映画タイトル</label>
                <input id="query" placeholder="映画タイトルで検索…" maxLength={200} value={query} onChange={event => setQuery(event.target.value)} />
                <fieldset className="year-range">
                    <legend className="sr-only">上映年で絞り込む（任意）</legend>
                    <label className="sr-only" htmlFor="year-from">開始年</label>
                    <input id="year-from" type="number" inputMode="numeric" min={1880} max={yearTo || 2100} step={1} placeholder="開始年" value={yearFrom} onChange={event => setYearFrom(event.target.value)} />
                    <span aria-hidden="true">〜</span>
                    <label className="sr-only" htmlFor="year-to">終了年</label>
                    <input id="year-to" type="number" inputMode="numeric" min={yearFrom || 1880} max={2100} step={1} placeholder="終了年" value={yearTo} onChange={event => setYearTo(event.target.value)} />
                </fieldset>
                <button className="primary" disabled={loading}>検索する</button>
            </form>
            <h2>{title}</h2>
            {!loading && !error && !demo && title === 'いま注目の映画' && <p className="muted">人気上位約500本から毎回ランダムに選んだ100本です（上映年を指定した場合はその範囲から選びます）。</p>}
            {!loading && !error && results.length > 0 && (
                <div className="recommend-box">
                    <p className="eyebrow">A PICK FOR YOU</p>
                    {jev ? (
                        <>
                            <p>今の気分を書くと、この一覧の未登録作品（{unsavedCount}本）からJevが1本を選びます。</p>
                            <form onSubmit={handleRecommend}>
                                <label className="sr-only" htmlFor="mood">今の気分</label>
                                <textarea id="mood" maxLength={500} required placeholder="たとえば、温かい気持ちになれる映画。少し笑えて、余韻が残るもの。" value={mood} onChange={event => setMood(event.target.value)} />
                                <button className="primary" disabled={recommending || unsavedCount === 0}>Jevに選んでもらう ↗</button>
                            </form>
                            <p className="muted">評価・メモ・気分と一覧の作品情報は、提案時にTypeSafe AIへ送信されます。</p>
                        </>
                    ) : <p className="muted">Jevによる提案には、サーバーにTypeSafe AIのAPIキーを設定してください。</p>}
                    <div className="grid" aria-busy={recommending}>
                        {recommending && <p className="empty" role="status">Jevが次の一本を選んでいます…</p>}
                        {recommendError && <p className="empty error" role="alert">{recommendError}</p>}
                        {suggestion && <CandidateCard movie={suggestion} saved={isSaved(suggestion)} onAdd={onAdd} />}
                    </div>
                </div>
            )}
            <div className="grid" aria-busy={loading}>
                {loading ? <p className="empty" role="status">映画を探しています…</p>
                    : error ? <p className="empty error" role="alert">{error}</p>
                    : results.length ? results.map(movie => <CandidateCard key={movie.id} movie={movie} saved={isSaved(movie)} onAdd={onAdd} />)
                    : <p className="empty">該当する映画が見つかりませんでした。別のタイトルや上映年で検索してください。</p>}
            </div>
        </section>
    );
}
