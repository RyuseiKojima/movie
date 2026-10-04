import { useCallback, useState } from 'react';
import { LibraryPanel } from './components/LibraryPanel';
import { RecommendPanel } from './components/RecommendPanel';
import { SearchPanel } from './components/SearchPanel';
import { useLibrary } from './hooks/useLibrary';
import type { Movie, Tab } from './types';

const tabs: { id: Tab; label: string }[] = [
    { id: 'search', label: '映画を探す' },
    { id: 'library', label: '私の映画棚' },
    { id: 'recommend', label: 'Jevで次の一本' }
];

export function App() {
    const [notice, setNotice] = useState('');
    const notify = useCallback((message: string) => setNotice(message), []);
    const [tab, setTab] = useState<Tab>('search');
    const [demo, setDemo] = useState(false);
    const { library, has, add, update, remove } = useLibrary(notify);

    function selectTab(next: Tab) {
        setTab(next);
        notify(demo ? 'デモモード：実際の検索・提案にはAPIキーを設定してください。' : '');
    }

    function handleAdd(movie: Movie) {
        add(movie);
        notify(`「${movie.title}」を登録しました。`);
    }

    return (
        <>
            <header><a className="brand" href="/">◉ Movie Shelf</a><span>映画と、あなたの記録。</span></header>
            <main>
                <section className="hero">
                    <p className="eyebrow">YOUR PERSONAL CINEMA</p>
                    <h1>次の一本も、<br />忘れたくない一本も。</h1>
                    <p>気になる映画を見つけて、あなただけの映画棚に。</p>
                </section>
                <nav aria-label="表示切替">
                    {tabs.map(({ id, label }) => (
                        <button key={id} className={tab === id ? 'active' : undefined} onClick={() => selectTab(id)}>
                            {label}{id === 'library' && <span id="count">{library.length}</span>}
                        </button>
                    ))}
                </nav>
                <p id="notice" role="status">{notice}</p>
                <SearchPanel hidden={tab !== 'search'} notify={notify} onConfig={setDemo} isSaved={has} onAdd={handleAdd} />
                <LibraryPanel hidden={tab !== 'library'} library={library} onUpdate={update} onRemove={remove} />
                <RecommendPanel hidden={tab !== 'recommend'} library={library} notify={notify} isSaved={has} onAdd={handleAdd} />
            </main>
            <footer>
                <p>映画情報：<a href="https://www.themoviedb.org/" target="_blank" rel="noreferrer">TMDB</a></p>
                <img src="https://files.readme.io/29c6fee-blue_short.svg" alt="The Movie Database" width="130" />
                <p>This product uses the TMDB API but is not endorsed or certified by TMDB.</p>
            </footer>
        </>
    );
}
