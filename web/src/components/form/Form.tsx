import type { FC, FormEvent, ReactNode } from 'react';

interface FormProps {
  // eslint-disable-next-line @typescript-eslint/no-deprecated
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  children: ReactNode;
  className?: string;
}

const Form: FC<FormProps> = ({ onSubmit, children, className }) => {
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault(); // Prevent default form submission
        onSubmit(event);
      }}
      className={` ${className ?? ''}`} // Default spacing between form fields
    >
      {children}
    </form>
  );
};

export default Form;
