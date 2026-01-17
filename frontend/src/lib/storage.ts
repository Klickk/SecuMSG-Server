const DB_NAME = "secumsg-storage";
const STORE_NAME = "kv";
const SECURE_STORE_NAME = "secure";
// Increment DB_VERSION whenever we change the schema (e.g., add new stores)
const DB_VERSION = 3;

let dbPromise: Promise<IDBDatabase> | null = null;

async function getDb(): Promise<IDBDatabase> {
  if (dbPromise) {
    return dbPromise;
  }

  if (typeof indexedDB === "undefined") {
    throw new Error("IndexedDB is not available in this environment");
  }

  dbPromise = new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION);

    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME);
      }
      if (!db.objectStoreNames.contains(SECURE_STORE_NAME)) {
        db.createObjectStore(SECURE_STORE_NAME);
      }
    };

    request.onsuccess = () => {
      const db = request.result;
      // If the store is missing (e.g., a stale/corrupted DB), recreate the DB.
      if (
        !db.objectStoreNames.contains(STORE_NAME) ||
        !db.objectStoreNames.contains(SECURE_STORE_NAME)
      ) {
        db.close();
        const deleteReq = indexedDB.deleteDatabase(DB_NAME);
        deleteReq.onerror = () => reject(deleteReq.error);
        deleteReq.onsuccess = () => {
          dbPromise = null;
          // Retry opening; the next call will hit onupgradeneeded and create the store.
          getDb().then(resolve).catch(reject);
        };
        return;
      }
      resolve(db);
    };
    request.onblocked = () => {
      dbPromise = null;
      reject(new Error("IndexedDB open blocked. Close other tabs and retry."));
    };
    request.onerror = () => reject(request.error);
  });

  return dbPromise;
}

export async function wipeDatabaseIfExists(): Promise<void> {
  if (typeof indexedDB === "undefined") {
    return;
  }

  // Close existing handles so deletion is not blocked.
  if (dbPromise) {
    dbPromise
      .then((existing) => existing.close())
      .catch(() => {
        // Ignore—if the previous open failed, we still want to retry deletion.
      });
  }
  dbPromise = null;

  await new Promise<void>((resolve, reject) => {
    const deleteReq = indexedDB.deleteDatabase(DB_NAME);
    deleteReq.onerror = () => reject(deleteReq.error);
    deleteReq.onblocked = () => resolve();
    deleteReq.onsuccess = () => resolve();
  });
}

export async function setItem(key: string, value: string): Promise<void> {
  await setItemInStore(STORE_NAME, key, value);
}

export async function getItem(key: string): Promise<string | null> {
  return getItemFromStore(STORE_NAME, key);
}

export async function removeItem(key: string): Promise<void> {
  await removeItemFromStore(STORE_NAME, key);
}

export async function setItemInStore(
  storeName: string,
  key: string,
  value: string
): Promise<void> {
  const db = await getDb();
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(storeName, "readwrite");
    const store = tx.objectStore(storeName);
    store.put(value, key);
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

export async function getItemFromStore(
  storeName: string,
  key: string
): Promise<string | null> {
  const db = await getDb();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(storeName, "readonly");
    const store = tx.objectStore(storeName);
    const req = store.get(key);
    req.onsuccess = () => resolve(req.result ?? null);
    req.onerror = () => reject(req.error);
  });
}

export async function removeItemFromStore(
  storeName: string,
  key: string
): Promise<void> {
  const db = await getDb();
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(storeName, "readwrite");
    const store = tx.objectStore(storeName);
    store.delete(key);
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

export async function listKeysInStore(
  storeName: string,
  prefix?: string
): Promise<string[]> {
  const db = await getDb();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(storeName, "readonly");
    const store = tx.objectStore(storeName);
    const results: string[] = [];
    const request = store.openCursor();
    request.onsuccess = () => {
      const cursor = request.result;
      if (!cursor) {
        resolve(results);
        return;
      }
      if (typeof cursor.key === "string") {
        if (!prefix || cursor.key.startsWith(prefix)) {
          results.push(cursor.key);
        }
      }
      cursor.continue();
    };
    request.onerror = () => reject(request.error);
    tx.onabort = () => reject(tx.error);
  });
}

export const STORE_NAMES = {
  plaintext: STORE_NAME,
  secure: SECURE_STORE_NAME,
};
