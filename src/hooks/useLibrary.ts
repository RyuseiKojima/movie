import { useCallback, useEffect, useState } from 'react';
import { isSameMovie, type Movie, type SavedMovie } from '../types';

const STORAGE_KEY = 'movie-shelf';

function load(): SavedMovie[] {
    try {
        const data: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]');
        return Array.isArray(data) ? data : [];
    } catch { return []; }
}

export function useLibrary(notify: (message: string) => void) {
    const [library, setLibrary] = useState<SavedMovie[]>(load);

    useEffect(() => {
        try { localStorage.setItem(STORAGE_KEY, JSON.stringify(library)); }
        catch { notify('保存できませんでした。ブラウザの保存容量をご確認ください。'); }
    }, [library, notify]);

    const has = useCallback((movie: Movie) => library.some(item => isSameMovie(item, movie)), [library]);
    const add = useCallback((movie: Movie) => {
        setLibrary(items => items.some(item => isSameMovie(item, movie)) ? items : [...items, { ...movie, status: 'want', rating: 0, note: '' }]);
    }, []);
    const update = useCallback((movie: SavedMovie, changes: Partial<SavedMovie>) => {
        setLibrary(items => items.map(item => isSameMovie(item, movie) ? { ...item, ...changes } : item));
    }, []);
    const remove = useCallback((movie: SavedMovie) => {
        setLibrary(items => items.filter(item => !isSameMovie(item, movie)));
    }, []);

    return { library, has, add, update, remove };
}
