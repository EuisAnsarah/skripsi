import streamlit as st
import pandas as pd
from rank_bm25 import BM25Okapi

# Read CSV
df = pd.read_csv('after3.csv').dropna(subset=['body'])

# Tokenize the corpus
tokenized_corpus = [(doc + " " + title).split() for doc, title in zip(df['body'],df['title'])]

# Create BM25 object
bm25 = BM25Okapi(tokenized_corpus)

# Define function to search and display results
def search(query, corpus, top_n=10):
    tokenized_query = query.lower().split()
    print(tokenized_query)
    top_results = bm25.get_top_n(tokenized_query, corpus, n=top_n)
    results = []
    for doc in top_results:
        index = corpus.index(doc)
        title = df.iloc[index]['title']
        link = df.iloc[index]['link']
        score = bm25.get_scores(tokenized_query)[index]
        results.append((title, link, score))
    return results

# Streamlit app
st.title('BM25 Search Engine')

query = st.text_input('Enter your search query:')
if st.button('Search'):
    if query:
        st.write("Top Results:")
        top_results = search(query, tokenized_corpus)
        for i, result in enumerate(top_results, start=1):
            if result[2] > 0:
                st.write(f"{i}. Judul: {result[0]}")
                st.write(f"   Tautan: {result[1]}")
                st.write(f"   Skor: {result[2]}")