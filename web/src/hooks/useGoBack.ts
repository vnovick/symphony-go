import { useNavigate } from 'react-router';

const useGoBack = () => {
  const navigate = useNavigate();

  const goBack = () => {
    const historyIdx = (window.history.state as { idx?: number } | null)?.idx ?? 0;
    if (historyIdx > 0) {
      void navigate(-1); // Go back to the previous page
    } else {
      void navigate('/'); // Redirect to home if no history exists
    }
  };

  return goBack;
};

export default useGoBack;
