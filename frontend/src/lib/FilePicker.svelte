<script lang="ts">
  interface Props {
    label: string;
    description: string;
    file?: File | null;
  }

  let { label, description, file = $bindable<File | null>(null) }: Props = $props();
  let previewURL = $state('');

  $effect(() => {
    if (!file) {
      previewURL = '';
      return;
    }
    const objectURL = URL.createObjectURL(file);
    previewURL = objectURL;
    return () => URL.revokeObjectURL(objectURL);
  });

  function selectFile(event: Event) {
    const input = event.currentTarget;
    if (input instanceof HTMLInputElement) {
      file = input.files?.[0] ?? null;
    }
  }
</script>

<label class="file-picker">
  <input type="file" accept="image/jpeg,image/png" onchange={selectFile} />
  {#if file && previewURL}
    <img class="file-preview" src={previewURL} alt={`Vista previa: ${file.name}`} />
  {:else}
    <span class="upload-mark" aria-hidden="true">+</span>
  {/if}
  <span class="file-picker-copy">
    <strong>{file ? file.name : label}</strong>
    <small>{file ? 'Imagen lista · JPEG o PNG' : description}</small>
  </span>
  <span class="choose-file">{file ? 'Cambiar' : 'Elegir imagen'}</span>
</label>
