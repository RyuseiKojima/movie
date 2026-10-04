import { useState, type FormEvent } from 'react';
import { recommendMovie } from '../api';
import type { Movie, SavedMovie } from '../types';
import { CandidateCard } from './MovieCard';

interface Props {
    hidden: boolean;
    library: SavedMovie[];
    notify: (message: string) => void;
    isSaved: (movie: Movie) => boolean;
    onAdd: (movie: Movie) => void;
}

export function RecommendPanel({ hidden, library, notify, isSaved, onAdd }: Props) {
    const [mood, setMood] = useState('');
    const [suggestion, setSuggestion] = useState<Movie | null>(null);
    const [loading, setLoading] = useState(false);

    async function handleSubmit(event: FormEvent) {
        event.preventDefault();
        setLoading(true);
        setSuggestion(null);
        notify('Jevが次の一本を選んでいます…');
        try {
            const result = await recommendMovie(mood, library);
            setSuggestion(result.movie);
            notify(`Jevが選んだ一本です。モデルの確信度：${Math.round(result.confidence * 100)}%（満足度の保証ではありません）`);
        } catch (error) { notify((error as Error).message); }
        finally { setLoading(false); }
    }

    return (
        <section id="recommend-panel" hidden={hidden}>
            <div className="recommend-box">
                <p className="eyebrow">A PICK FOR YOU</p>
                <h2>今日は、どんな映画の気分？</h2>
                <p>鑑賞済みの評価・メモと今の気分から、Jevが候補を選びます。</p>
                <form onSubmit={handleSubmit}>
                    <label className="sr-only" htmlFor="mood">今の気分</label>
                    <textarea id="mood" maxLength={500} required placeholder="たとえば、温かい気持ちになれる映画。少し笑えて、余韻が残るもの。" value={mood} onChange={event => setMood(event.target.value)} />
                    <button className="primary" disabled={loading}>Jevに選んでもらう ↗</button>
                </form>
                <p className="muted">評価・メモ・気分は提案時にTypeSafe AIへ送信されます。</p>
            </div>
            <div className="grid">
                {suggestion && <CandidateCard movie={suggestion} saved={isSaved(suggestion)} onAdd={onAdd} />}
            </div>
        </section>
    );
}
