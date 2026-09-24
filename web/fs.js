// A node-style globalThis.fs for Go's js/wasm port, loaded before wasm_exec.js: the game's config.rec and
// saves live in memory and are mirrored to IndexedDB; feeds/*.ans go to the page (window.parent.hhFeed) instead.
// ponytail: whole files in memory and one IDB put per closed file; fine for saves of a few MB.
(() => {
	const files = new Map(); // path -> Uint8Array
	const dirs = new Set([""]);
	const fds = new Map(); // fd -> {path, pos, dirty, append}
	let nextFd = 3;
	const dec = new TextDecoder();
	const err = (code) => Object.assign(new Error(code), { code });
	const norm = (p) => p.replace(/^(\.?\/)+/, "").replace(/\/+$/, "").replace(/\/\.\//g, "/");
	const dirOf = (p) => p.includes("/") ? p.slice(0, p.lastIndexOf("/")) : "";
	const feedOf = (p) => /^feeds\/(\w+)\.ans$/.exec(p)?.[1];
	const stat = (p) => {
		const dir = dirs.has(p);
		if (!dir && !files.has(p)) throw err("ENOENT");
		const size = dir ? 0 : files.get(p).length, t = Date.now();
		return { dev: 0, ino: 0, mode: dir ? 0o40755 : 0o100644, nlink: 1, uid: 0, gid: 0, rdev: 0, size, blksize: 4096,
			blocks: Math.ceil(size / 512), atimeMs: t, mtimeMs: t, ctimeMs: t, isDirectory: () => dir };
	};

	let db;
	const store = (mode) => db?.transaction("f", mode).objectStore("f");
	const persist = (p) => { if (!p.startsWith("feeds/")) files.has(p) ? store("readwrite")?.put(files.get(p), p) : store("readwrite")?.delete(p); };
	const feed = (p, text, append) => { const n = feedOf(p); if (n) window.parent.hhFeed?.(n, text, append); };

	// Sync bodies; the node API is callback based.
	const ops = {
		open(p, flags) {
			p = norm(p);
			const C = globalThis.fs.constants;
			if (dirs.has(p)) return nextFd++, fds.set(nextFd - 1, { path: p, dir: true }), nextFd - 1;
			if (!files.has(p)) {
				if (!(flags & C.O_CREAT)) throw err("ENOENT");
				if (!dirs.has(dirOf(p))) throw err("ENOENT");
				files.set(p, new Uint8Array(0));
			} else if (flags & C.O_EXCL) throw err("EEXIST");
			if (flags & C.O_TRUNC) files.set(p, new Uint8Array(0));
			fds.set(nextFd, { path: p, pos: 0, append: !!(flags & C.O_APPEND) });
			return nextFd++;
		},
		close(fd) {
			const f = fds.get(fd);
			fds.delete(fd);
			if (f?.dirty) persist(f.path);
		},
		read(fd, buf, off, len, pos) {
			const f = fds.get(fd), data = files.get(f.path), at = pos ?? f.pos;
			const n = Math.max(0, Math.min(len, data.length - at));
			buf.set(data.subarray(at, at + n), off);
			if (pos == null) f.pos += n;
			return n;
		},
		write(fd, buf, off, len, pos) {
			const chunk = buf.subarray(off, off + len);
			if (fd === 1 || fd === 2) { (fd === 1 ? console.log : console.error)(dec.decode(chunk)); return len; }
			const f = fds.get(fd);
			if (f.path === "feeds/log.ans") { feed(f.path, dec.decode(chunk), true); return len; } // only grows: not kept
			let data = files.get(f.path);
			const at = f.append ? data.length : pos ?? f.pos;
			if (at + len > data.length) { const d = new Uint8Array(at + len); d.set(data); data = d; files.set(f.path, data); }
			data.set(chunk, at);
			if (pos == null) f.pos = at + len;
			f.dirty = true;
			return len;
		},
		fstat: (fd) => stat(fds.get(fd).path),
		stat: (p) => stat(norm(p)),
		lstat: (p) => stat(norm(p)),
		mkdir(p) {
			p = norm(p);
			if (dirs.has(p) || files.has(p)) throw err("EEXIST");
			if (!dirs.has(dirOf(p))) throw err("ENOENT");
			dirs.add(p);
		},
		readdir(p) {
			p = norm(p);
			if (!dirs.has(p)) throw err("ENOENT");
			const pre = p ? p + "/" : "";
			return [...new Set([...files.keys(), ...dirs].filter((k) => k && k.startsWith(pre)).map((k) => k.slice(pre.length).split("/")[0]))];
		},
		unlink(p) { p = norm(p); if (!files.delete(p)) throw err("ENOENT"); persist(p); },
		rmdir(p) { dirs.delete(norm(p)); },
		rename(from, to) {
			from = norm(from); to = norm(to);
			if (!files.has(from)) throw err("ENOENT");
			files.set(to, files.get(from)); files.delete(from);
			persist(from); persist(to);
			if (feedOf(to)) feed(to, dec.decode(files.get(to)), false);
		},
		ftruncate(fd, len) { const f = fds.get(fd); files.set(f.path, files.get(f.path).slice(0, len)); f.dirty = true; },
		truncate(p, len) { p = norm(p); files.set(p, files.get(p).slice(0, len)); persist(p); },
	};
	const fs = { constants: { O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_EXCL: 128, O_TRUNC: 512, O_APPEND: 1024, O_DIRECTORY: -1 },
		writeSync: (fd, buf) => ops.write(fd, buf, 0, buf.length, null) };
	for (const [name, fn] of Object.entries(ops)) {
		fs[name] = (...args) => {
			const cb = args.pop();
			try { cb(null, fn(...args)); } catch (e) { cb(e.code ? e : err("EIO")); }
		};
	}
	for (const name of ["fsync", "utimes", "chmod", "fchmod", "chown", "fchown", "lchown"]) fs[name] = (...args) => args.pop()(null);
	for (const name of ["link", "symlink", "readlink"]) fs[name] = (...args) => args.pop()(err("ENOSYS"));
	globalThis.fs = fs;

	// Load every stored file before the game starts; a first visit gets defaults.rec as its config.rec.
	globalThis.fsReady = new Promise((resolve) => {
		const req = indexedDB.open("heavenandhell", 1);
		req.onupgradeneeded = () => req.result.createObjectStore("f");
		req.onerror = () => resolve(); // private window: plays, doesn't save
		req.onsuccess = () => {
			db = req.result;
			const cur = store("readonly").openCursor();
			cur.onsuccess = () => {
				const c = cur.result;
				if (!c) return resolve();
				files.set(c.key, c.value);
				for (let d = dirOf(c.key); d; d = dirOf(d)) dirs.add(d);
				c.continue();
			};
			cur.onerror = () => resolve();
		};
	}).then(async () => {
		dirs.add("feeds");
		if (!files.has("config.rec")) files.set("config.rec", new Uint8Array(await (await fetch("defaults.rec")).arrayBuffer()));
	});
})();
